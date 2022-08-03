package applier

import (
	"context"

	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/platform-operators/api/v1alpha1"
	"github.com/openshift/platform-operators/internal/sourcer"
)

const (
	plainProvisionerID    = "core.rukpak.io/plain"
	registryProvisionerID = "core.rukpak.io/registry"
)

type bdApplier struct {
	client.Client
}

func NewBundleDeploymentHandler(c client.Client) Applier {
	return &bdApplier{
		Client: c,
	}
}

func (a *bdApplier) Apply(ctx context.Context, po *v1alpha1.PlatformOperator, b *sourcer.Bundle) error {
	bi := &rukpakv1alpha1.BundleDeployment{}
	bi.SetName(po.GetName())
	controllerRef := metav1.NewControllerRef(po, po.GroupVersionKind())

	_, err := controllerutil.CreateOrUpdate(ctx, a.Client, bi, func() error {
		bi.SetOwnerReferences([]metav1.OwnerReference{*controllerRef})
		bi.Spec = *buildBundleDeployment(b.Image)
		return nil
	})
	return err
}

// buildBundleDeployment is responsible for taking a name and image to create an embedded BundleDeployment
func buildBundleDeployment(image string) *rukpakv1alpha1.BundleDeploymentSpec {
	return &rukpakv1alpha1.BundleDeploymentSpec{
		ProvisionerClassName: plainProvisionerID,
		// TODO(tflannag): Investigate why the metadata key is empty when this
		// resource has been created on cluster despite the field being omitempty.
		Template: &rukpakv1alpha1.BundleTemplate{
			Spec: rukpakv1alpha1.BundleSpec{
				// TODO(tflannag): Dynamically determine provisioner ID based on bundle
				// format? Do we need an API for discovering available provisioner IDs
				// in the cluster, and to map those ID(s) to bundle formats?
				ProvisionerClassName: registryProvisionerID,
				Source: rukpakv1alpha1.BundleSource{
					Type: rukpakv1alpha1.SourceTypeImage,
					Image: &rukpakv1alpha1.ImageSource{
						Ref: image,
					},
				},
			},
		},
	}
}
