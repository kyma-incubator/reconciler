package test

import (
	"os"
	"strings"
)

const (
	EnvIntegrationTests = "RECONCILER_INTEGRATION_TESTS"
)

func RunIntegrationTests() bool {
	expensiveTests, ok := os.LookupEnv(EnvIntegrationTests)
	if !ok {
		return false
	}
	return expensiveTests == "1" || strings.ToLower(expensiveTests) == "true"
}

func EnableIntegrationTests() error {
	return os.Setenv(EnvIntegrationTests, "true")
}

func DisableIntegrationTests() error {
	return os.Unsetenv(EnvIntegrationTests)
}
