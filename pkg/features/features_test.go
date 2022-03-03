package features

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestFeatures(t *testing.T) {

	type testCases struct {
		description    string
		funcToTest     func() bool
		envVar         string
		envVarValue    string
		expectedResult bool
	}

	for _, testCase := range []testCases{
		{description: "Workerpool Occupancy set to 1", funcToTest: WorkerpoolOccupancyTrackingEnabled, envVar: workerpoolOccupancyTrackingEnvVar, envVarValue: "1", expectedResult: true},
		{description: "Workerpool Occupancy set to lowercase true", funcToTest: WorkerpoolOccupancyTrackingEnabled, envVar: workerpoolOccupancyTrackingEnvVar, envVarValue: "true", expectedResult: true},
		{description: "Workerpool Occupancy set to TrUe; not lowercase", funcToTest: WorkerpoolOccupancyTrackingEnabled, envVar: workerpoolOccupancyTrackingEnvVar, envVarValue: "TrUe", expectedResult: true},
		{description: "Workerpool Occupancy set to 0", funcToTest: WorkerpoolOccupancyTrackingEnabled, envVar: workerpoolOccupancyTrackingEnvVar, envVarValue: "0", expectedResult: false},
		{description: "Workerpool Occupancy not set", funcToTest: WorkerpoolOccupancyTrackingEnabled, envVar: workerpoolOccupancyTrackingEnvVar, envVarValue: "", expectedResult: false},
		{description: "Processing Duration Metric set to 1", funcToTest: ProcessingDurationMetricsEnabled, envVar: processingDurationMetricEnvVar, envVarValue: "1", expectedResult: true},
		{description: "Processing Duration Metric set to lowercase true", funcToTest: ProcessingDurationMetricsEnabled, envVar: processingDurationMetricEnvVar, envVarValue: "true", expectedResult: true},
		{description: "Processing Duration Metric set to TrUe; not lowercase", funcToTest: ProcessingDurationMetricsEnabled, envVar: processingDurationMetricEnvVar, envVarValue: "TrUe", expectedResult: true},
		{description: "Processing Duration Metric set to 0", funcToTest: ProcessingDurationMetricsEnabled, envVar: processingDurationMetricEnvVar, envVarValue: "0", expectedResult: false},
		{description: "Processing Duration Metric not set", funcToTest: ProcessingDurationMetricsEnabled, envVar: processingDurationMetricEnvVar, envVarValue: "", expectedResult: false},
	} {
		test := testCase
		t.Run(test.description, func(t *testing.T) {
			if test.envVarValue != "" {
				err := os.Setenv(test.envVar, test.envVarValue)
				require.NoError(t, err)
			}

			actualResult := test.funcToTest()
			require.Equal(t, test.expectedResult, actualResult)

			err := os.Unsetenv(test.envVar)
			require.NoError(t, err)
		})
	}

}
