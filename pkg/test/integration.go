package test

import (
	"os"
	"strings"
	"testing"
)

const (
	EnvIntegrationTests = "RECONCILER_INTEGRATION_TESTS"
)

func IntegrationTest(t *testing.T) {
	//if !isIntegrationTestEnabled() {
	//	t.Skipf("Integration tests disabled: skipping parts of test case '%s'", t.Name())
	//}
}

func isIntegrationTestEnabled() bool {
	expensiveTests, ok := os.LookupEnv(EnvIntegrationTests)
	return ok && (expensiveTests == "1" || strings.ToLower(expensiveTests) == "true")
}

func EnableIntegrationTests() error {
	return os.Setenv(EnvIntegrationTests, "true")
}

func DisableIntegrationTests() error {
	return os.Unsetenv(EnvIntegrationTests)
}
