package applier

import (
	"context"

	"github.com/openshift/platform-operators/api/v1alpha1"
	"github.com/openshift/platform-operators/internal/sourcer"
)

type Applier interface {
	Apply(context.Context, *v1alpha1.PlatformOperator, *sourcer.Bundle) error
}
