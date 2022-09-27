package clusteroperator

import "time"

const (
	CoreResourceName               = "platform-operators-core"
	AggregateResourceName          = "platform-operators-aggregated"
	DefaultUnavailabilityThreshold = 5 * time.Minute
)
