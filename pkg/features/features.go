package features

import (
	"os"
	"strings"
)

type Feature int

const (
	ProcessingDurationMetric Feature = iota + 1
	WorkerpoolOccupancyTracking
	LogIstioOperator
	DebugLogForSpecificOperations
)

// define the mapping between feature name and env var name
var featureEnVarMap = map[Feature]string{
	ProcessingDurationMetric:      "PROCESSING_DURATION_METRICS_ENABLED",
	WorkerpoolOccupancyTracking:   "WORKERPOOL_OCCUPANCY_TRACKING_ENABLED",
	LogIstioOperator:              "LOG_ISTIO_OPERATOR",
	DebugLogForSpecificOperations: "DEBUG_LOGGING_FOR_SPECIFIC_OPERATIONS",
}

func Enabled(feature Feature) bool {
	return checkEnvVar(envVar(feature))
}

func envVar(feature Feature) string {
	return featureEnVarMap[feature]
}

func checkEnvVar(envVar string) bool {
	enabled := os.Getenv(envVar)
	if strings.ToLower(enabled) == "true" || enabled == "1" {
		return true
	}
	return false
}
