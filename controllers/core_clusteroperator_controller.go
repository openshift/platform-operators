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

	configv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	"github.com/openshift/platform-operators/internal/clusteroperator"
)

type CoreClusterOperatorReconciler struct {
	client.Client
	ReleaseVersion  string
	SystemNamespace string
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
		core = &configv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusteroperator.CoreResourceName,
			},
			Status: configv1.ClusterOperatorStatus{},
		}
		return ctrl.Result{}, r.Create(ctx, core)
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

	poList := &platformv1alpha1.PlatformOperatorList{}
	if err := r.List(ctx, poList); err != nil {
		return ctrl.Result{}, err
	}

	coBuilder.WithAvailable(configv1.ConditionTrue, fmt.Sprintf("The platform operator manager is available at %s", r.ReleaseVersion), clusteroperator.ReasonAsExpected)
	coBuilder.WithProgressing(configv1.ConditionFalse, "")

	return ctrl.Result{}, nil
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
