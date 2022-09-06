package util

import (
	"context"
	"fmt"
	"time"

	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
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

type POStatusErrors struct {
	FailingPOs    []*platformv1alpha1.PlatformOperator
	FailingErrors []error
}

// InspectPlatformOperators iterates over all the POs on the cluster
// and determines whether a PO is in a failing state by inspecting its status.
// A nil return value indicates no errors were found with the POs provided.
func InspectPlatformOperators(POList *platformv1alpha1.PlatformOperatorList) *POStatusErrors {
	POstatuses := new(POStatusErrors)

	for _, po := range POList.Items {
		po := po.DeepCopy()
		status := po.Status

		for _, condition := range status.Conditions {
			if condition.Reason == platformtypes.ReasonApplyFailed {
				POstatuses.FailingPOs = append(POstatuses.FailingPOs, po)
				POstatuses.FailingErrors = append(POstatuses.FailingErrors, fmt.Errorf("%s is failing: %q", po.GetName(), condition.Reason))
			}
		}
	}

	// check if any POs were populated in the POStatusErrors type
	if len(POstatuses.FailingPOs) > 0 || len(POstatuses.FailingErrors) > 0 {
		return POstatuses
	}

	return nil
}
