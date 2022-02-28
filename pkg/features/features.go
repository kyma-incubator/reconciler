package features

import "os"

const processingDurationMetricEnvVar = "PROCESSING_DURATION_METRICS_ENABLED"
const workerpoolOccupancyTrackingEnvVar = "WORKERPOOL_OCCUPANCY_TRACKING_ENABLED"

func ProcessingDurationMetricsEnabled() bool {
	enabled := os.Getenv(processingDurationMetricEnvVar)
	if enabled == "true" || enabled == "1" {
		return true
	}
	return false
}

func WorkerpoolOccupancyTrackingEnabled() bool {
	enabled := os.Getenv(workerpoolOccupancyTrackingEnvVar)
	if enabled == "true" || enabled == "1" {
		return true
	}
	return false
}
