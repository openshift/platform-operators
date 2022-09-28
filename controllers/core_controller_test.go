package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
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
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: clusteroperator.CoreResourceName},
			})
			Expect(err).ToNot(BeNil())
			Expect(res).To(Equal(ctrl.Result{}))

			By("ensuring the clusteroperator has been updated with D=T")
			co := &configv1.ClusterOperator{}
			err = r.Get(context.Background(), types.NamespacedName{Name: clusteroperator.CoreResourceName}, co)
			Expect(err).To(BeNil())

			GinkgoT().Logf("waiting for the clusteroperator status to report an unavailable status: %v", co.Status.Conditions)

			degraded := clusteroperator.FindStatusCondition(co.Status.Conditions, configv1.OperatorDegraded)
			Expect(degraded).ToNot(BeNil())
			Expect(degraded.Status).To(Equal(configv1.ConditionTrue))
		})

		It("should eventually return an available=false status after exceeding the threshold", func() {
			Eventually(func() (bool, error) {
				res, err := r.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: clusteroperator.CoreResourceName},
				})
				Expect(err).ToNot(BeNil())
				Expect(res).To(Equal(ctrl.Result{}))

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
