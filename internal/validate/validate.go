package validate

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
)

// UniquePackage checks that the provided PlatformOperator contains a unique spec.package.name
// when compared to the other PlatformOperators existing on cluster.
func UniquePackage(ctx context.Context, cli client.Client, po *platformv1alpha1.PlatformOperator) error {
	existingPlatformOperators := &platformv1alpha1.PlatformOperatorList{}
	if err := cli.List(ctx, existingPlatformOperators); err != nil {
		return err
	}

	for _, existingPO := range existingPlatformOperators.Items {
		// check whether we're processing the same PlatformOperator as the parameter
		if existingPO.GetName() == po.GetName() {
			continue
		}

		if existingPO.Spec.Package.Name == po.Spec.Package.Name {
			return fmt.Errorf(
				"%v spec.package.name is not unique, conflicts with PlatformOperator %v",
				po.Name,
				existingPO.Name,
			)
		}
	}

	return nil
}
