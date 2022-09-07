package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	platformtypes "github.com/openshift/platform-operators/api/v1alpha1"
)

const (
	aggregateCOName = "platform-operators-aggregated"
)

var _ = Describe("aggregated clusteroperator controller", func() {
	var (
		ctx context.Context
	)
	BeforeEach(func() {
		ctx = context.Background()
	})

	When("no POs have been installed on the cluster", func() {
		It("should consistently report a healthy CO status back to the CVO", func() {
			Consistently(func() (*configv1.ClusterOperatorStatusCondition, error) {
				co := &configv1.ClusterOperator{}
				if err := c.Get(ctx, types.NamespacedName{Name: aggregateCOName}, co); err != nil {
					return nil, err
				}
				return FindStatusCondition(co.Status.Conditions, configv1.OperatorAvailable), nil
			}).Should(And(
				Not(BeNil()),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ClusterStatusConditionType { return c.Type }, Equal(configv1.OperatorAvailable)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ConditionStatus { return c.Status }, Equal(configv1.ConditionTrue)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Reason }, Equal("NoPOsFound")),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Message }, ContainSubstring("No POs are present in the cluster")),
			))
		})
	})

	When("installing a series of POs successfully", func() {
		var (
			po *platformv1alpha1.PlatformOperator
		)
		BeforeEach(func() {
			po = &platformv1alpha1.PlatformOperator{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cert-manager",
				},
				Spec: platformv1alpha1.PlatformOperatorSpec{
					Package: platformv1alpha1.Package{
						Name: "openshift-cert-manager-operator",
					},
				},
			}
			Expect(c.Create(ctx, po)).To(BeNil())
		})
		AfterEach(func() {
			Expect(c.Delete(ctx, po)).To(BeNil())
		})

		It("should eventually result in a successful application", func() {
			Eventually(func() (*metav1.Condition, error) {
				if err := c.Get(ctx, client.ObjectKeyFromObject(po), po); err != nil {
					return nil, err
				}
				return meta.FindStatusCondition(po.Status.Conditions, platformtypes.TypeApplied), nil
			}).Should(And(
				Not(BeNil()),
				WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(platformtypes.TypeApplied)),
				WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionTrue)),
				WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(platformtypes.ReasonInstallSuccessful)),
				WithTransform(func(c *metav1.Condition) string { return c.Message }, ContainSubstring("Successfully applied the desired olm.bundle content")),
			))
		})

		It("should eventually report a healthy CO status back to the CVO", func() {
			Eventually(func() (*configv1.ClusterOperatorStatusCondition, error) {
				co := &configv1.ClusterOperator{}
				if err := c.Get(ctx, types.NamespacedName{Name: aggregateCOName}, co); err != nil {
					return nil, err
				}
				return FindStatusCondition(co.Status.Conditions, configv1.OperatorAvailable), nil
			}).Should(And(
				Not(BeNil()),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ClusterStatusConditionType { return c.Type }, Equal(configv1.OperatorAvailable)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ConditionStatus { return c.Status }, Equal(configv1.ConditionTrue)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Reason }, Equal("POsHealthy")),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Message }, ContainSubstring("All POs in a successful state")),
			))
		})
	})

	When("a failing PO has been encountered", func() {
		var (
			po *platformv1alpha1.PlatformOperator
		)
		BeforeEach(func() {
			po = &platformv1alpha1.PlatformOperator{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "non-existent-operator",
				},
				Spec: platformv1alpha1.PlatformOperatorSpec{
					Package: platformv1alpha1.Package{
						Name: "non-existent-operator",
					},
				},
			}
			Expect(c.Create(ctx, po)).To(BeNil())
		})
		AfterEach(func() {
			Expect(c.Delete(ctx, po)).To(BeNil())
		})

		It("should eventually result in a failed attempt at sourcing that non-existent package", func() {
			Eventually(func() (*metav1.Condition, error) {
				if err := c.Get(ctx, client.ObjectKeyFromObject(po), po); err != nil {
					return nil, err
				}
				return meta.FindStatusCondition(po.Status.Conditions, platformtypes.TypeApplied), nil
			}).Should(And(
				Not(BeNil()),
				WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(platformtypes.TypeApplied)),
				WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionUnknown)),
				WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(platformtypes.ReasonSourceFailed)),
				WithTransform(func(c *metav1.Condition) string { return c.Message }, ContainSubstring("failed to find candidate")),
			))
		})

		It("should eventually report an unvailable CO status back to the CVO", func() {
			Eventually(func() (*configv1.ClusterOperatorStatusCondition, error) {
				co := &configv1.ClusterOperator{}
				if err := c.Get(ctx, types.NamespacedName{Name: aggregateCOName}, co); err != nil {
					return nil, err
				}
				return FindStatusCondition(co.Status.Conditions, configv1.OperatorAvailable), nil
			}).Should(And(
				Not(BeNil()),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ClusterStatusConditionType { return c.Type }, Equal(configv1.OperatorAvailable)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ConditionStatus { return c.Status }, Equal(configv1.ConditionFalse)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Reason }, Equal("POError")),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Message }, ContainSubstring("encountered the failing")),
			))
		})
	})

	When("there's a mixture of failing and successful POs deployed on the cluster", func() {
		var (
			invalid *platformv1alpha1.PlatformOperator
			valid   *platformv1alpha1.PlatformOperator
		)
		BeforeEach(func() {
			invalid = &platformv1alpha1.PlatformOperator{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "non-existent-operator",
				},
				Spec: platformv1alpha1.PlatformOperatorSpec{
					Package: platformv1alpha1.Package{
						Name: "non-existent-operator",
					},
				},
			}
			Expect(c.Create(ctx, invalid)).To(BeNil())

			valid = &platformv1alpha1.PlatformOperator{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cert-manager",
				},
				Spec: platformv1alpha1.PlatformOperatorSpec{
					Package: platformv1alpha1.Package{
						Name: "openshift-cert-manager-operator",
					},
				},
			}
			Expect(c.Create(ctx, valid)).To(BeNil())
		})
		AfterEach(func() {
			Expect(c.Delete(ctx, invalid)).To(BeNil())
			Expect(c.Delete(ctx, valid)).To(BeNil())
		})

		It("should eventually result in a failed attempt at sourcing that non-existent package", func() {
			Eventually(func() (*metav1.Condition, error) {
				if err := c.Get(ctx, client.ObjectKeyFromObject(invalid), invalid); err != nil {
					return nil, err
				}
				return meta.FindStatusCondition(invalid.Status.Conditions, platformtypes.TypeApplied), nil
			}).Should(And(
				Not(BeNil()),
				WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(platformtypes.TypeApplied)),
				WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionUnknown)),
				WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(platformtypes.ReasonSourceFailed)),
				WithTransform(func(c *metav1.Condition) string { return c.Message }, ContainSubstring("failed to find candidate")),
			))
		})

		It("should eventually result in a successful application", func() {
			Eventually(func() (*metav1.Condition, error) {
				if err := c.Get(ctx, client.ObjectKeyFromObject(valid), valid); err != nil {
					return nil, err
				}
				return meta.FindStatusCondition(valid.Status.Conditions, platformtypes.TypeApplied), nil
			}).Should(And(
				Not(BeNil()),
				WithTransform(func(c *metav1.Condition) string { return c.Type }, Equal(platformtypes.TypeApplied)),
				WithTransform(func(c *metav1.Condition) metav1.ConditionStatus { return c.Status }, Equal(metav1.ConditionTrue)),
				WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(platformtypes.ReasonInstallSuccessful)),
				WithTransform(func(c *metav1.Condition) string { return c.Message }, ContainSubstring("Successfully applied the desired olm.bundle content")),
			))
		})

		It("should eventually report an unvailable CO status back to the CVO", func() {
			Eventually(func() (*configv1.ClusterOperatorStatusCondition, error) {
				co := &configv1.ClusterOperator{}
				if err := c.Get(ctx, types.NamespacedName{Name: aggregateCOName}, co); err != nil {
					return nil, err
				}
				return FindStatusCondition(co.Status.Conditions, configv1.OperatorAvailable), nil
			}).Should(And(
				Not(BeNil()),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ClusterStatusConditionType { return c.Type }, Equal(configv1.OperatorAvailable)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ConditionStatus { return c.Status }, Equal(configv1.ConditionFalse)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Reason }, Equal("POError")),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Message }, ContainSubstring("encountered the failing")),
			))
		})
	})
})

// FindStatusCondition finds the conditionType in conditions.
// Note: manually vendored from o/library-go/pkg/config/clusteroperator/v1helpers/status.go.
func FindStatusCondition(conditions []configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
