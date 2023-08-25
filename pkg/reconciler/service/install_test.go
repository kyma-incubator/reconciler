package service

import (
	"fmt"
	"os"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestInstall(t *testing.T) {
	tests := []struct {
		envVars  map[string]string
		expected []string
	}{
		{
			envVars: map[string]string{
				"IM_INVALID": "true",
				fmt.Sprintf("%s%s", model.SkippedComponentEnvVarPrefix, "ABC"):  "tRue",
				fmt.Sprintf("%s%s", model.SkippedComponentEnvVarPrefix, "DE_F"): "1",
				fmt.Sprintf("%s%s", model.SkippedComponentEnvVarPrefix, "GH"):   "0",
				fmt.Sprintf("%s%s", model.SkippedComponentEnvVarPrefix, "XYZ"):  "truee",
			},
			expected: []string{"abc", "de-f"},
		},
	}

	for _, testCase := range tests {
		install := NewInstall(logger.NewTestLogger(t))
		for envKey, envValue := range testCase.envVars {
			os.Setenv(envKey, envValue)
		}
		got := install.skippedComps()
		require.Len(t, got, len(testCase.expected))
		require.ElementsMatch(t, testCase.expected, got)
	}
}
