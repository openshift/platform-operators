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
	"errors"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/openshift/platform-operators/internal/checker"
	"github.com/openshift/platform-operators/internal/clusteroperator"
)

// TODO(tflannag): Appropriately set the "Progressing" status condition
// type during cluster upgrade events.
// FIXME(tflannag): I'm seeing unit test flakes where we're bumping
// the lastTransistionTime value despite being in the same state as
// before which is a bug.

var (
	errUnavailable = errors.New("platform operators manager has failed an availability check")
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
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusteroperators,verbs=get;list;watch
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
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	defer func() {
		if err := coWriter.UpdateStatus(ctx, core, coBuilder.GetStatus()); err != nil {
			log.Error(err, "error updating CO status")
		}
	}()

	// Add the default ClusterOperator status configurations to the builder instance.
	clusteroperator.SetDefaultStatusConditions(coBuilder, r.ReleaseVersion)
	clusteroperator.SetDefaultRelatedObjects(coBuilder, r.SystemNamespace)

	// check whether the we're currently passing the availability checks. note: in
	// the case where we were previously failing these checks, and we now have passed
	// them, the expectation is that we're now setting an A=T state and purging any
	// D=T states.
	if available := r.CheckAvailability(ctx, core); available {
		coBuilder.WithAvailable(configv1.ConditionTrue, fmt.Sprintf("The platform operator manager is available at %s", r.ReleaseVersion), clusteroperator.ReasonAsExpected)
		coBuilder.WithProgressing(configv1.ConditionFalse, "")
		coBuilder.WithDegraded(configv1.ConditionFalse)
		return ctrl.Result{}, nil
	}

	log.Info("manager failed an availability check")
	// we failed the availability checks, and now need to determine whether we to set
	// D=T if this is the first time we've failed an availability check to avoid
	// prematurely setting A=F during transient events.
	if meetsDegradedStatusCriteria(core) {
		log.Info("setting degraded=true since this is the first violation")
		// avoid stomping on the current A=T status condition value if that
		// status condition type was previously set.
		available := clusteroperator.FindStatusCondition(core.Status.Conditions, configv1.OperatorAvailable)
		if available != nil && available.Status == configv1.ConditionTrue {
			coBuilder.WithAvailable(configv1.ConditionTrue, available.Message, available.Reason)
		}
		coBuilder.WithDegraded(configv1.ConditionTrue)
		return ctrl.Result{}, errUnavailable
	}
	// check whether the time spent in the the D=T state has exceeded the configured
	// threshold, and mark the ClusterOperator as unavailable.
	if timeInDegradedStateExceedsThreshold(ctx, core, r.Now(), r.AvailabilityThreshold) {
		log.Info("adjusted timestamp has exceeded unavailability theshold: setting A=F and P=F")

		coBuilder.WithAvailable(configv1.ConditionFalse, "Exceeded platform operator availability timeout", "ExceededUnavailabilityThreshold")
		coBuilder.WithProgressing(configv1.ConditionFalse, "Exceeded platform operator availability timeout")
		coBuilder.WithDegraded(configv1.ConditionTrue)
	}
	return ctrl.Result{}, errUnavailable
}

func meetsDegradedStatusCriteria(co *configv1.ClusterOperator) bool {
	degraded := clusteroperator.FindStatusCondition(co.Status.Conditions, configv1.OperatorDegraded)
	return degraded == nil || degraded.Status != configv1.ConditionTrue
}

func timeInDegradedStateExceedsThreshold(
	ctx context.Context,
	co *configv1.ClusterOperator,
	startTime time.Time,
	threshold time.Duration,
) bool {
	degraded := clusteroperator.FindStatusCondition(co.Status.Conditions, configv1.OperatorDegraded)
	if degraded == nil {
		return false
	}
	lastEncounteredTime := degraded.LastTransitionTime
	adjustedTime := lastEncounteredTime.Add(threshold)

	logr.FromContext(ctx).Info("checking whether time spent in degraded state has exceeded the configured threshold",
		"threshold", threshold.String(),
		"current", startTime.String(),
		"last", lastEncounteredTime.String(),
		"adjusted", adjustedTime.String(),
	)
	// check whether we've exceeded the availability threshold by comparing
	// the currently recorded lastTransistionTime, adding the threshold buffer, and
	// verifying whether that adjusted timestamp is less than the current clock timestamp.
	return adjustedTime.Before(startTime)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CoreClusterOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1.ClusterOperator{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			// TODO(tflannag): Investigate using using a label selector to avoid caching
			// all clusteroperator resources, and then filtering for the "core" clusteroperator
			// resource from that shared cache.
			return object.GetName() == clusteroperator.CoreResourceName
		}))).
		Complete(r)
}
