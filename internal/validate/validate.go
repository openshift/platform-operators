package validate

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
)

func UniquePackage(ctx context.Context, cli client.Client, po *platformv1alpha1.PlatformOperator) error {
	existingPlatformOperators := &platformv1alpha1.PlatformOperatorList{}
	if err := cli.List(ctx, existingPlatformOperators); err != nil {
		return err
	}

	for _, otherPO := range existingPlatformOperators.Items {
		if otherPO.Spec.Package.Name == po.Spec.Package.Name {
			return fmt.Errorf(
				"%v spec.package.name is not unique, conflicts with PlatformOperator %v",
				po.Name,
				otherPO.Name,
			)
		}
	}

	return nil
}
