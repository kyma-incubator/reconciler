package features

import (
	"os"
	"strings"
)

const processingDurationMetricEnvVar = "PROCESSING_DURATION_METRICS_ENABLED"
const workerpoolOccupancyTrackingEnvVar = "WORKERPOOL_OCCUPANCY_TRACKING_ENABLED"

func ProcessingDurationMetricsEnabled() bool {
	return checkEnvVar(processingDurationMetricEnvVar)
}

func WorkerpoolOccupancyTrackingEnabled() bool {
	return checkEnvVar(workerpoolOccupancyTrackingEnvVar)
}

func checkEnvVar(envVar string) bool {
	enabled := os.Getenv(envVar)
	if strings.ToLower(enabled) == "true" || enabled == "1" {
		return true
	}
	return false
}
