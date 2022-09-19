package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/platform-operators/internal/clusteroperator"
)

var _ = Describe("core clusteroperator controller", func() {
	var (
		ctx context.Context
	)
	BeforeEach(func() {
		ctx = context.Background()
	})

	When("the core clusteroperator exists", func() {
		It("should consistently report Available=True", func() {
			Consistently(func() (*configv1.ClusterOperatorStatusCondition, error) {
				co := &configv1.ClusterOperator{}
				if err := c.Get(ctx, types.NamespacedName{Name: clusteroperator.CoreResourceName}, co); err != nil {
					return nil, err
				}
				return FindStatusCondition(co.Status.Conditions, configv1.OperatorAvailable), nil
			}).Should(And(
				Not(BeNil()),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ClusterStatusConditionType { return c.Type }, Equal(configv1.OperatorAvailable)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) configv1.ConditionStatus { return c.Status }, Equal(configv1.ConditionTrue)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Reason }, Equal(clusteroperator.ReasonAsExpected)),
				WithTransform(func(c *configv1.ClusterOperatorStatusCondition) string { return c.Message }, ContainSubstring("The platform operator manager is available")),
			))
		})
	})

	When("the core clusteroperator has been deleted", func() {
		It("should be recreated", func() {
			By("getting the core clusteroperator resource")
			co := &configv1.ClusterOperator{}
			err := c.Get(ctx, types.NamespacedName{Name: clusteroperator.CoreResourceName}, co)
			Expect(err).To(BeNil())

			By("recording the origin UUID of the core resource")
			originalUID := co.GetUID()

			By("deleting the core clusteroperator resource")
			Expect(c.Delete(ctx, co)).To(Succeed())

			By("verifying the core clusteroperator resources' UID has changed")
			Eventually(func() (types.UID, error) {
				err := c.Get(ctx, client.ObjectKeyFromObject(co), co)
				return co.GetUID(), err
			}).ShouldNot(Equal(originalUID))
		})
	})
})
