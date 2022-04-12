package features

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestFeatures(t *testing.T) {

	type testCases struct {
		description    string
		feature        Feature
		envVarValue    string
		expectedResult bool
	}

	for _, testCase := range []testCases{
		{description: "Workerpool Occupancy set to 1", feature: WorkerpoolOccupancyTracking, envVarValue: "1", expectedResult: true},
		{description: "Workerpool Occupancy set to lowercase true", feature: WorkerpoolOccupancyTracking, envVarValue: "true", expectedResult: true},
		{description: "Workerpool Occupancy set to TrUe; not lowercase", feature: WorkerpoolOccupancyTracking, envVarValue: "TrUe", expectedResult: true},
		{description: "Workerpool Occupancy set to 0", feature: WorkerpoolOccupancyTracking, envVarValue: "0", expectedResult: false},
		{description: "Workerpool Occupancy not set", feature: WorkerpoolOccupancyTracking, envVarValue: "", expectedResult: false},
		{description: "Processing Duration Metric set to 1", feature: WorkerpoolOccupancyTracking, envVarValue: "1", expectedResult: true},
		{description: "Processing Duration Metric set to lowercase true", feature: WorkerpoolOccupancyTracking, envVarValue: "true", expectedResult: true},
		{description: "Processing Duration Metric set to TrUe; not lowercase", feature: WorkerpoolOccupancyTracking, envVarValue: "TrUe", expectedResult: true},
		{description: "Processing Duration Metric set to 0", feature: WorkerpoolOccupancyTracking, envVarValue: "0", expectedResult: false},
		{description: "Processing Duration Metric not set", feature: WorkerpoolOccupancyTracking, envVarValue: "", expectedResult: false},
	} {
		test := testCase
		t.Run(test.description, func(t *testing.T) {
			if test.envVarValue != "" {
				err := os.Setenv(envVar(test.feature), test.envVarValue)
				require.NoError(t, err)
			}

			actualResult := Enabled(test.feature)
			require.Equal(t, test.expectedResult, actualResult)

			err := os.Unsetenv(envVar(test.feature))
			require.NoError(t, err)
		})
	}

}
