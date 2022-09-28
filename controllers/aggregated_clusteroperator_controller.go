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

	configv1 "github.com/openshift/api/config/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	"github.com/openshift/platform-operators/internal/clusteroperator"
	"github.com/openshift/platform-operators/internal/util"
)

type AggregatedClusterOperatorReconciler struct {
	client.Client
	ReleaseVersion  string
	SystemNamespace string
}

//+kubebuilder:rbac:groups=platform.openshift.io,resources=platformoperators,verbs=list
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusteroperators,verbs=get;list;watch
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusteroperators/status,verbs=update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *AggregatedClusterOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContext(ctx)
	log.Info("reconciling request", "req", req.NamespacedName)
	defer log.Info("finished reconciling request", "req", req.NamespacedName)

	coBuilder := clusteroperator.NewBuilder()
	coWriter := clusteroperator.NewWriter(r.Client)

	aggregatedCO := &configv1.ClusterOperator{}
	if err := r.Get(ctx, req.NamespacedName, aggregatedCO); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	defer func() {
		if err := coWriter.UpdateStatus(ctx, aggregatedCO, coBuilder.GetStatus()); err != nil {
			log.Error(err, "error updating CO status")
		}
	}()

	// Set the default CO status conditions: Progressing=True, Degraded=False, Available=False
	coBuilder.WithProgressing(configv1.ConditionTrue, "")
	coBuilder.WithDegraded(configv1.ConditionFalse)
	coBuilder.WithAvailable(configv1.ConditionFalse, "", "")
	coBuilder.WithVersion("operator", r.ReleaseVersion)
	coBuilder.WithRelatedObject("", "namespaces", "", r.SystemNamespace)
	coBuilder.WithRelatedObject("platform.openshift.io", "platformoperators", "", "")

	poList := &platformv1alpha1.PlatformOperatorList{}
	if err := r.List(ctx, poList); err != nil {
		return ctrl.Result{}, err
	}
	if len(poList.Items) == 0 {
		// No POs on cluster, everything is fine
		coBuilder.WithAvailable(configv1.ConditionTrue, "No POs are present in the cluster", "NoPOsFound")
		coBuilder.WithProgressing(configv1.ConditionFalse, "No POs are present in the cluster")
		return ctrl.Result{}, nil
	}

	// check whether any of the underlying PO resources are reporting
	// any failing status states, and update the aggregate CO resource
	// to reflect those failing PO resources.
	if statusErrorCheck := util.InspectPlatformOperators(poList); statusErrorCheck != nil {
		coBuilder.WithAvailable(configv1.ConditionFalse, statusErrorCheck.Error(), "POError")
		return ctrl.Result{}, nil
	}
	coBuilder.WithAvailable(configv1.ConditionTrue, "All POs in a successful state", "POsHealthy")
	coBuilder.WithProgressing(configv1.ConditionFalse, "All POs in a successful state")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AggregatedClusterOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1.ClusterOperator{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == clusteroperator.AggregateResourceName
		}))).
		Watches(&source.Kind{Type: &platformv1alpha1.PlatformOperator{}}, handler.EnqueueRequestsFromMapFunc(util.RequeueClusterOperator(mgr.GetClient(), clusteroperator.AggregateResourceName))).
		Complete(r)
}
