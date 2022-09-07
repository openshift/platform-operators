package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rukpakv1alpha1 "github.com/operator-framework/rukpak/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	platformtypes "github.com/openshift/platform-operators/api/v1alpha1"
)

var _ = Describe("platform operators controller", func() {
	var (
		ctx context.Context
	)
	BeforeEach(func() {
		ctx = context.Background()
	})
	When("sourcing content from the redhat-operators catalog source", func() {
		When("a platformoperator has been created", func() {
			var (
				po *platformv1alpha1.PlatformOperator
			)
			BeforeEach(func() {
				po = &platformv1alpha1.PlatformOperator{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "prometheus-operator",
					},
					Spec: platformv1alpha1.PlatformOperatorSpec{
						Package: platformv1alpha1.Package{
							Name: "prometheus-operator",
						},
					},
				}
				Expect(c.Create(ctx, po)).To(BeNil())
			})
			AfterEach(func() {
				Expect(c.Delete(ctx, po)).To(BeNil())
			})
			It("should generate a Bundle Deployment with a metadata.Name that matches the platformoperator's metadata.Name", func() {
				Eventually(func() error {
					bi := &rukpakv1alpha1.BundleDeployment{}
					return c.Get(ctx, types.NamespacedName{Name: po.GetName()}, bi)
				}).Should(Succeed())
			})
			It("should generate a Bundle Deployment that contains the different unique provisioner ID", func() {
				Eventually(func() bool {
					bi := &rukpakv1alpha1.BundleDeployment{}
					if err := c.Get(ctx, types.NamespacedName{Name: po.GetName()}, bi); err != nil {
						return false
					}
					return bi.Spec.Template.Spec.ProvisionerClassName != bi.Spec.ProvisionerClassName
				}).Should(BeTrue())
			})
			It("should choose the highest olm.bundle semver available in the catalog", func() {
				Eventually(func() bool {
					bi := &rukpakv1alpha1.BundleDeployment{}
					if err := c.Get(ctx, types.NamespacedName{Name: po.GetName()}, bi); err != nil {
						return false
					}
					return bi.Spec.Template.Spec.Source.Image.Ref == "quay.io/operatorhubio/prometheus:v0.47.0"
				}).Should(BeTrue())
			})
			It("should result in a successful application", func() {
				Eventually(func() (*metav1.Condition, error) {
					if err := c.Get(ctx, client.ObjectKeyFromObject(po), po); err != nil {
						return nil, err
					}
					return meta.FindStatusCondition(po.Status.Conditions, platformtypes.TypeApplied), nil
				}).Should(And(
					Not(BeNil()),
					WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(platformtypes.TypeApplied)),
					WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionTrue)),
					WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(platformtypes.ReasonApplySuccessful)),
					WithTransform(func(c *metav1.Condition) string { return c.Message }, ContainSubstring("Successfully applied the desired olm.bundle content")),
				))
			})
			It("should result in a successful installation", func() {
				Eventually(func() (*metav1.Condition, error) {
					bi := &rukpakv1alpha1.BundleDeployment{}
					if err := c.Get(ctx, types.NamespacedName{Name: po.GetName()}, bi); err != nil {
						return nil, err
					}
					if bi.Status.ActiveBundle == "" {
						return nil, fmt.Errorf("waiting for bundle name to be populated")
					}
					return meta.FindStatusCondition(bi.Status.Conditions, rukpakv1alpha1.TypeInstalled), nil
				}).Should(And(
					Not(BeNil()),
					WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(rukpakv1alpha1.TypeInstalled)),
					WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionTrue)),
					WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(rukpakv1alpha1.ReasonInstallationSucceeded)),
					WithTransform(func(c *metav1.Condition) string { return c.Message }, ContainSubstring("instantiated bundle")),
				))
			})
		})
	})
})
