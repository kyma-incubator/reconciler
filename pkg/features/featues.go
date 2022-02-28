package features

import "os"

const processingDurationEnvVar = "PROCESSING_DURATION_METRICS_ENABLED"

func ProcessingDurationMetricsEnabled() bool {
	enabled := os.Getenv(processingDurationEnvVar)
	if enabled == "true" || enabled == "1" {
		return true
	} else {
		return false
	}
}
