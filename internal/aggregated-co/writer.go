package aggregated_co

import (
	"context"
	"reflect"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
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

// UpdateStatus updates the clusteroperator object with the new status specified.
func (w *Writer) UpdateStatus(ctx context.Context, existing *configv1.ClusterOperator, newStatus *configv1.ClusterOperatorStatus) error {
	if newStatus == nil || existing == nil {
		panic("input specified is <nil>")
	}

	existingStatus := existing.Status.DeepCopy()
	if reflect.DeepEqual(existingStatus, newStatus) {
		return nil
	}

	existing.Status = *newStatus
	if _, err := w.client.ClusterOperators().UpdateStatus(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}
