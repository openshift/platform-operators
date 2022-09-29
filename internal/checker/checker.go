package checker

import (
	"context"

	configv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
)

type Checker interface {
	CheckAvailability(context.Context, *configv1.ClusterOperator) bool
}

type ListChecker struct {
	client.Client
}

func (c ListChecker) CheckAvailability(ctx context.Context, _ *configv1.ClusterOperator) bool {
	poList := &platformv1alpha1.PlatformOperatorList{}
	return c.List(ctx, poList) == nil
}

type NoopChecker struct {
	Available bool
}

func (c NoopChecker) CheckAvailability(_ context.Context, _ *configv1.ClusterOperator) bool {
	return c.Available
}
