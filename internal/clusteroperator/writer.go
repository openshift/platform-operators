package clusteroperator

import (
	"context"
	"reflect"

	configv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// From https://github.com/operator-framework/operator-lifecycle-manager/pkg/lib/operatorstatus/writer.go

// NewWriter returns a new instance of Writer.
func NewWriter(client client.Client) *Writer {
	return &Writer{
		Client: client,
	}
}

// Writer encapsulates logic for cluster operator object API. It is used to
// update ClusterOperator resource.
type Writer struct {
	client.Client
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
	if err := w.Status().Update(ctx, existing); err != nil {
		return err
	}
	return nil
}
