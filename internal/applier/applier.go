package applier

import (
	"context"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	"github.com/openshift/platform-operators/internal/sourcer"
)

type Applier interface {
	Apply(context.Context, *platformv1alpha1.PlatformOperator, *sourcer.Bundle) error
}
