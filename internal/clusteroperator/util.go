package clusteroperator

import (
	configv1 "github.com/openshift/api/config/v1"
)

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

// SetDefaultStatusConditions adds the default ClusterOperator status conditions to
// the current Builder parameter. Those default status conditions are
// Progressing=True, Degraded=False, and Available=False.
func SetDefaultStatusConditions(builder *Builder, version string) {
	builder.WithProgressing(configv1.ConditionTrue, "")
	builder.WithDegraded(configv1.ConditionFalse)
	builder.WithAvailable(configv1.ConditionFalse, "", "")
	builder.WithVersion("operator", version)
}

// SetDefaultRelatedObjects adds the default ClusterOperator related object
// configurations to the Builder parameter.
func SetDefaultRelatedObjects(builder *Builder, namespace string) {
	builder.WithRelatedObject("", "namespaces", "", namespace)
	builder.WithRelatedObject("platform.openshift.io", "platformoperators", "", "")
	builder.WithRelatedObject("core.rukpak.io", "bundles", "", "")
	builder.WithRelatedObject("core.rukpak.io", "bundledeployments", "", "")
}
