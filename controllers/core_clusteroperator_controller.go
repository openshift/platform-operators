/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/openshift/platform-operators/internal/checker"
	"github.com/openshift/platform-operators/internal/clusteroperator"
)

type CoreClusterOperatorReconciler struct {
	client.Client
	clock.Clock
	checker.Checker

	ReleaseVersion        string
	SystemNamespace       string
	AvailabilityThreshold time.Duration
}

//+kubebuilder:rbac:groups=platform.openshift.io,resources=platformoperators,verbs=list
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusteroperators,verbs=get;list;watch;create
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusteroperators/status,verbs=update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *CoreClusterOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContext(ctx)
	log.Info("reconciling request", "req", req.NamespacedName)
	defer log.Info("finished reconciling request", "req", req.NamespacedName)

	coBuilder := clusteroperator.NewBuilder()
	coWriter := clusteroperator.NewWriter(r.Client)

	core := &configv1.ClusterOperator{}
	if err := r.Get(ctx, req.NamespacedName, core); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		log.Info("core clusteroperator does not exist. recreating it...", "name", clusteroperator.CoreResourceName)
		return ctrl.Result{}, r.Create(ctx, r.newClusterOperator())
	}
	defer func() {
		if err := coWriter.UpdateStatus(ctx, core, coBuilder.GetStatus()); err != nil {
			log.Error(err, "error updating CO status")
		}
	}()

	// Set the default CO status conditions: Progressing=True, Degraded=False, Available=False
	coBuilder.WithProgressing(configv1.ConditionTrue, "")
	coBuilder.WithDegraded(configv1.ConditionFalse)
	coBuilder.WithAvailable(configv1.ConditionFalse, "", "")
	coBuilder.WithVersion("operator", r.ReleaseVersion)

	ensureRelatedObjects(coBuilder, r.SystemNamespace)

	log.Info("checking whether the platform operator manager is available")
	available, err := r.CheckAvailability(ctx, core)
	if err != nil {
		return ctrl.Result{}, err
	}
	if available {
		log.Info("manager is available")
		coBuilder.WithAvailable(configv1.ConditionTrue, fmt.Sprintf("The platform operator manager is available at %s", r.ReleaseVersion), clusteroperator.ReasonAsExpected)
		coBuilder.WithProgressing(configv1.ConditionFalse, "")
		return ctrl.Result{}, nil
	}

	log.Info("manager failed an availability check")
	// check whether we need to set D=T if this is the first time we've failed
	// an availability check.
	degraded := clusteroperator.FindStatusCondition(core.Status.Conditions, configv1.OperatorDegraded)
	if degraded == nil || degraded.Status != configv1.ConditionTrue {
		log.Info("setting degraded=true since this is the first violation")
		// in the case that we've already recorded A=T, and this is the first time we've failed an
		// availability check, then set D=T and retain the currently recorded A=? value to avoid
		// prematurely setting A=F during transient events.
		available := clusteroperator.FindStatusCondition(core.Status.Conditions, configv1.OperatorAvailable)
		if available != nil && available.Status == configv1.ConditionTrue {
			coBuilder.WithAvailable(configv1.ConditionTrue, available.Message, available.Reason)
		}
		coBuilder.WithDegraded(configv1.ConditionTrue)
		return ctrl.Result{}, err
	}

	currentTime := r.Clock.Now()
	lastEncounteredTime := degraded.LastTransitionTime
	adjustedTime := lastEncounteredTime.Add(r.AvailabilityThreshold)
	log.Info("checking whether time spent in degraded state has exceeded the configured threshold",
		"threshold", r.AvailabilityThreshold.String(),
		"current", currentTime.String(),
		"last", lastEncounteredTime.String(),
		"adjusted", adjustedTime.String(),
	)

	// check whether we've exceeded the availability threshold by comparing
	// the currently recorded lastTransistionTime, adding the threshold buffer, and
	// verifying whether that adjusted timestamp is less than the current clock timestamp.
	if adjustedTime.Before(currentTime) {
		log.Info("adjusted timestamp has exceeded unavailability theshold: setting A=F and P=F")
		// we've exceeded the configured threshold. note: A=F is the default value
		// here, so we need to only set P=T and retain the already existing D=T value.
		coBuilder.WithAvailable(configv1.ConditionFalse, "Exceeded platform operator availability timeout", "ExceededUnavailabilityThreshold")
		coBuilder.WithProgressing(configv1.ConditionFalse, "Exceeded platform operator availability timeout")
	}
	return ctrl.Result{}, err
}

func (r *CoreClusterOperatorReconciler) newClusterOperator() *configv1.ClusterOperator {
	return &configv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusteroperator.CoreResourceName,
		},
		Status: configv1.ClusterOperatorStatus{},
	}
}

func ensureRelatedObjects(builder *clusteroperator.Builder, namespace string) {
	builder.WithRelatedObject("", "namespaces", "", namespace)
	builder.WithRelatedObject("platform.openshift.io", "platformoperators", "", "")
	builder.WithRelatedObject("core.rukpak.io", "bundles", "", "")
	builder.WithRelatedObject("core.rukpak.io", "bundledeployments", "", "")
}

// SetupWithManager sets up the controller with the Manager.
func (r *CoreClusterOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1.ClusterOperator{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == clusteroperator.CoreResourceName
		}))).
		Complete(r)
}
