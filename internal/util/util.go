package util

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerror "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	platformtypes "github.com/openshift/platform-operators/api/v1alpha1"
)

var (
	ShortRequeue = ctrl.Result{RequeueAfter: time.Second * 5}
)

func RequeuePlatformOperators(cl client.Client) handler.MapFunc {
	return func(object client.Object) []reconcile.Request {
		poList := &platformv1alpha1.PlatformOperatorList{}
		if err := cl.List(context.Background(), poList); err != nil {
			return nil
		}

		var requests []reconcile.Request
		for _, po := range poList.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: po.GetName(),
				},
			})
		}
		return requests
	}
}

func RequeueBundleDeployment(c client.Client) handler.MapFunc {
	return func(obj client.Object) []reconcile.Request {
		bi := obj.(*rukpakv1alpha1.BundleDeployment)

		poList := &platformv1alpha1.PlatformOperatorList{}
		if err := c.List(context.Background(), poList); err != nil {
			return nil
		}

		var requests []reconcile.Request
		for _, po := range poList.Items {
			po := po

			for _, ref := range bi.GetOwnerReferences() {
				if ref.Name == po.GetName() {
					requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&po)})
				}
			}
		}
		return requests
	}
}

func RequeueClusterOperator(c client.Client, name string) handler.MapFunc {
	return func(obj client.Object) []reconcile.Request {
		co := &configv1.ClusterOperator{}

		if err := c.Get(context.Background(), types.NamespacedName{Name: name}, co); err != nil {
			return nil
		}
		return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(co)}}
	}
}

// InspectPlatformOperators iterates over all the POs on the cluster
// and determines whether a PO is in a failing state by inspecting its status.
// A nil return value indicates no errors were found with the POs provided.
func InspectPlatformOperators(POList *platformv1alpha1.PlatformOperatorList) error {
	var failingPOs []error
	for _, po := range POList.Items {
		if err := inspectPlatformOperator(po); err != nil {
			failingPOs = append(failingPOs, err)
		}
	}
	if len(failingPOs) > 0 {
		return utilerror.NewAggregate(failingPOs)
	}
	return nil
}

// inspectPlatformOperator is responsible for inspecting an individual platform
// operator resource, and determining whether it's reporting any failing conditions.
// In the case that the PO resource is expressing failing states, then an error
// will be returned to reflect that.
func inspectPlatformOperator(po platformv1alpha1.PlatformOperator) error {
	applied := meta.FindStatusCondition(po.Status.Conditions, platformtypes.TypeApplied)
	if applied == nil {
		return buildPOFailureMessage(po.GetName(), platformtypes.ReasonApplyPending)
	}
	if applied.Status != metav1.ConditionTrue {
		return buildPOFailureMessage(po.GetName(), applied.Reason)
	}
	return nil
}

func buildPOFailureMessage(name, reason string) error {
	return fmt.Errorf("encountered the failing %s platform operator with reason %q", name, reason)
}
