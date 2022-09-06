package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	platformtypes "github.com/openshift/platform-operators/api/v1alpha1"
)

var _ = Describe("aggregated clusteroperator controller", func() {
	var (
		ctx context.Context
	)
	BeforeEach(func() {
		ctx = context.Background()
	})

	When("installing a series of POs successfully", func() {
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

		It("should eventually report a healthy CO status back to the CVO", func() {
			Eventually(func() (*configv1.ClusterOperatorStatusCondition, error) {
				co := &configv1.ClusterOperator{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(co), co); err != nil {
					return nil, err
				}
				for _, cond := range co.Status.Conditions {
					if cond.Type == configv1.OperatorAvailable {
						return &cond, nil
					}
					continue
				}
				return nil, nil
			}).Should(And(
				Not(BeNil()),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ClusterStatusConditionType { return c.Type }, Equal(configv1.OperatorAvailable)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ConditionStatus { return c.Status }, Equal(configv1.ConditionTrue)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Reason }, Equal("POsHealthy")),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Message }, ContainSubstring("All POs in a successful state")),
			))
		})
	})
})
