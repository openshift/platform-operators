package applier

import (
	rukpakv1alpha2 "github.com/operator-framework/rukpak/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
)

const (
	registryProvisionerID = "core-rukpak-io-registry"
)

func NewBundleDeployment(po *platformv1alpha1.PlatformOperator, image string) *rukpakv1alpha2.BundleDeployment {
	bd := &rukpakv1alpha2.BundleDeployment{}
	bd.SetName(po.GetName())

	controllerRef := metav1.NewControllerRef(po, po.GroupVersionKind())
	bd.SetOwnerReferences([]metav1.OwnerReference{*controllerRef})

	bd.Spec = buildBundleDeployment(image)
	return bd
}

// buildBundleDeployment is responsible for taking a name and image to create an embedded BundleDeployment
func buildBundleDeployment(image string) rukpakv1alpha2.BundleDeploymentSpec {
	return rukpakv1alpha2.BundleDeploymentSpec{
		ProvisionerClassName: registryProvisionerID,
		// TODO(tflannag): Investigate why the metadata key is empty when this
		// resource has been created on cluster despite the field being omitempty.
		Source: rukpakv1alpha2.BundleSource{
			Type: rukpakv1alpha2.SourceTypeImage,
			Image: &rukpakv1alpha2.ImageSource{
				Ref: image,
			},
		},
	}
}
