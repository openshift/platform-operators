package aggregated_co

import (
	"context"
	"reflect"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// From https://github.com/operator-framework/operator-lifecycle-manager/pkg/lib/operatorstatus/writer.go

// NewWriter returns a new instance of Writer.
func NewWriter(client configv1client.ConfigV1Interface) *Writer {
	return &Writer{
		client: client,
	}
}

// Writer encapsulates logic for cluster operator object API. It is used to
// update ClusterOperator resource.
type Writer struct {
	client configv1client.ConfigV1Interface
}

// EnsureExists ensures that the cluster operator resource exists with a default
// status that reflects expecting status.
func (w *Writer) EnsureExists(name string) (existing *configv1.ClusterOperator, err error) {
	existing, err = w.client.ClusterOperators().Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return
	}

	if !apierrors.IsNotFound(err) {
		return
	}

	co := &configv1.ClusterOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	existing, err = w.client.ClusterOperators().Create(context.TODO(), co, metav1.CreateOptions{})
	return
}

// UpdateStatus updates the clusteroperator object with the new status specified.
func (w *Writer) UpdateStatus(existing *configv1.ClusterOperator, newStatus *configv1.ClusterOperatorStatus) error {
	if newStatus == nil || existing == nil {
		panic("input specified is <nil>")
	}

	existingStatus := existing.Status.DeepCopy()
	if reflect.DeepEqual(existingStatus, newStatus) {
		return nil
	}

	existing.Status = *newStatus
	if _, err := w.client.ClusterOperators().UpdateStatus(context.TODO(), existing, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}
