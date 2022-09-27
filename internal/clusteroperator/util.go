package clusteroperator

import configv1 "github.com/openshift/api/config/v1"

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
