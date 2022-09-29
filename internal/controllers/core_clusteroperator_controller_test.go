package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openshift/platform-operators/internal/checker"
	"github.com/openshift/platform-operators/internal/clusteroperator"
)

var _ = Describe("Core ClusterOperator Controller", func() {
	var (
		ctx context.Context
	)
	BeforeEach(func() {
		ctx = context.Background()
	})

	When("the controller cannot successfully perform availability checks", func() {
		var (
			r      *CoreClusterOperatorReconciler
			c      client.Client
			cancel context.CancelFunc

			co *configv1.ClusterOperator
		)
		BeforeEach(func() {
			mgr, err := manager.New(cfg, manager.Options{
				MetricsBindAddress: "0",
				Scheme:             scheme,
			})
			Expect(err).ToNot(HaveOccurred())

			c = mgr.GetClient()

			r = &CoreClusterOperatorReconciler{
				Client: c,
				Clock:  clock.RealClock{},
				Checker: checker.NoopChecker{
					Available: false,
				},
				AvailabilityThreshold: 15 * time.Second,
			}

			ctx, cancel = context.WithCancel(context.Background())
			go func() { Expect(mgr.GetCache().Start(ctx)) }()
			Expect(mgr.GetCache().WaitForCacheSync(ctx)).To(BeTrue())

			co = clusteroperator.NewClusterOperator(clusteroperator.CoreResourceName)
			Expect(c.Create(ctx, co)).To(Succeed())
		})
		AfterEach(func() {
			Expect(c.Delete(ctx, co)).To(Succeed())
			cancel()
		})

		It("should result in the clusteroperator reporting a D=T after during the initial availability failures", func() {
			By("ensuring the next reconciliation returns an error")
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: clusteroperator.CoreResourceName},
			})
			Expect(err).ToNot(BeNil())

			By("ensuring the clusteroperator has been updated with D=T")
			Eventually(func() (bool, error) {
				co := &configv1.ClusterOperator{}
				if err = r.Get(context.Background(), types.NamespacedName{Name: clusteroperator.CoreResourceName}, co); err != nil {
					return false, err
				}

				degraded := clusteroperator.FindStatusCondition(co.Status.Conditions, configv1.OperatorDegraded)
				return degraded != nil && degraded.Status == configv1.ConditionTrue, nil
			}).Should(BeTrue())
		})

		It("should eventually return an available=false status after exceeding the threshold", func() {
			Eventually(func() (bool, error) {
				_, err := r.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: clusteroperator.CoreResourceName},
				})
				Expect(err).ToNot(BeNil())

				co := &configv1.ClusterOperator{}
				if err := r.Get(context.Background(), types.NamespacedName{Name: clusteroperator.CoreResourceName}, co); err != nil {
					return false, err
				}
				available := clusteroperator.FindStatusCondition(co.Status.Conditions, configv1.OperatorAvailable)
				return available == nil || available.Status != configv1.ConditionFalse, nil
			}).Should(BeTrue())
		})
	})
})

var _ = Describe("meetsDegradedStatusCriteria", func() {
	When("the co has an empty status", func() {
		var (
			co *configv1.ClusterOperator
		)
		BeforeEach(func() {
			co = clusteroperator.NewClusterOperator(clusteroperator.CoreResourceName)
		})
		It("should return true", func() {
			Expect(meetsDegradedStatusCriteria(co)).To(BeTrue())
		})
	})

	When("the co is reporting D=F status", func() {
		var (
			co *configv1.ClusterOperator
		)
		BeforeEach(func() {
			co = clusteroperator.NewClusterOperator(clusteroperator.CoreResourceName)
			SetStatusCondition(co, metav1.Condition{
				Type:   string(configv1.OperatorDegraded),
				Status: metav1.ConditionStatus(configv1.ConditionFalse),
			})
		})
		It("should return true", func() {
			Expect(meetsDegradedStatusCriteria(co)).To(BeTrue())
		})
	})

	When("the co is reporting a D=T status", func() {
		var (
			co *configv1.ClusterOperator
		)
		BeforeEach(func() {
			co = clusteroperator.NewClusterOperator(clusteroperator.CoreResourceName)
			SetStatusCondition(co, metav1.Condition{
				Type:   string(configv1.OperatorDegraded),
				Status: metav1.ConditionStatus(configv1.ConditionTrue),
			})
		})
		It("should return false", func() {
			Expect(meetsDegradedStatusCriteria(co)).To(BeFalse())
		})
	})
})

var _ = Describe("timeInDegradedStateExceedsThreshold", func() {
	var (
		ctx context.Context
	)
	BeforeEach(func() {
		ctx = context.Background()
	})
	When("the co has an empty status", func() {
		var (
			co *configv1.ClusterOperator
		)
		BeforeEach(func() {
			co = clusteroperator.NewClusterOperator(clusteroperator.CoreResourceName)
		})
		It("should return false", func() {
			Expect(timeInDegradedStateExceedsThreshold(ctx, co, time.Now(), 15*time.Second)).To(BeFalse())
		})
	})

	When("the co is reporting D=F status", func() {
		var (
			co *configv1.ClusterOperator
		)
		BeforeEach(func() {
			co = clusteroperator.NewClusterOperator(clusteroperator.CoreResourceName)
			SetStatusCondition(co, metav1.Condition{
				Type:   string(configv1.OperatorDegraded),
				Status: metav1.ConditionStatus(configv1.ConditionFalse),
			})
		})
		It("should return false", func() {
			Expect(timeInDegradedStateExceedsThreshold(ctx, co, time.Now(), 15*time.Second)).To(BeFalse())
		})
	})

	When("the co is reporting a D=T status that's before the threshold", func() {
		var (
			co *configv1.ClusterOperator
		)
		BeforeEach(func() {
			co = clusteroperator.NewClusterOperator(clusteroperator.CoreResourceName)
			SetStatusCondition(co, metav1.Condition{
				Type:   string(configv1.OperatorDegraded),
				Status: metav1.ConditionStatus(configv1.ConditionTrue),
			})
		})
		It("should return false", func() {
			Expect(timeInDegradedStateExceedsThreshold(ctx, co, time.Now(), 15*time.Second)).To(BeFalse())
		})
	})

	When("the co is reporting a D=T status that exceeds the threshold", func() {
		var (
			co    *configv1.ClusterOperator
			start time.Time
		)
		BeforeEach(func() {
			start = time.Now()

			co = clusteroperator.NewClusterOperator(clusteroperator.CoreResourceName)
			SetStatusCondition(co, metav1.Condition{
				Type:               string(configv1.OperatorDegraded),
				Status:             metav1.ConditionStatus(configv1.ConditionTrue),
				LastTransitionTime: metav1.NewTime(start.Add(15 * time.Second)),
			})
		})
		It("should return false", func() {
			Expect(timeInDegradedStateExceedsThreshold(ctx, co, start, 15*time.Second)).To(BeFalse())
		})
	})
})

func SetStatusCondition(co *configv1.ClusterOperator, cond metav1.Condition) {
	conditions := convertToMetaV1Conditions(co.Status.Conditions)
	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:    cond.Type,
		Status:  cond.Status,
		Reason:  cond.Reason,
		Message: cond.Message,
	})
	co.Status.Conditions = convertToClusterOperatorConditions(conditions)
}

func convertToMetaV1Conditions(in []configv1.ClusterOperatorStatusCondition) []metav1.Condition {
	out := make([]metav1.Condition, 0, len(in))
	for _, c := range in {
		out = append(out, metav1.Condition{
			Type:               string(c.Type),
			Status:             metav1.ConditionStatus(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		})
	}
	return out
}

func convertToClusterOperatorConditions(in []metav1.Condition) []configv1.ClusterOperatorStatusCondition {
	out := make([]configv1.ClusterOperatorStatusCondition, 0, len(in))
	for _, c := range in {
		out = append(out, configv1.ClusterOperatorStatusCondition{
			Type:               configv1.ClusterStatusConditionType(c.Type),
			Status:             configv1.ConditionStatus(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		})
	}
	return out
}
