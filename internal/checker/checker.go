package checker

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1 "github.com/openshift/api/config/v1"
	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
)

type Checker interface {
	CheckAvailability(context.Context, *configv1.ClusterOperator) (bool, error)
}

type ListChecker struct {
	client.Client
}

func (c *ListChecker) CheckAvailability(ctx context.Context, _ *configv1.ClusterOperator) (bool, error) {
	poList := &platformv1alpha1.PlatformOperatorList{}
	if err := c.List(ctx, poList); err != nil {
		return false, nil
	}
	return true, nil
}

type NoopChecker struct{}

func (c *NoopChecker) CheckAvailability(_ context.Context, _ *configv1.ClusterOperator) (bool, error) {
	return false, nil
}
