package test

import (
	"os"
	"strings"
)

const (
	EnvExpensiveTests = "RECONCILER_EXPENSIVE_TESTS"
)

func RunExpensiveTests() bool {
	expensiveTests, ok := os.LookupEnv(EnvExpensiveTests)
	if !ok {
		return false
	}
	return expensiveTests == "1" || strings.ToLower(expensiveTests) == "true"
}

func EnableExpensiveTests() {
	os.Setenv(EnvExpensiveTests, "true")
}

func DisableExpensiveTests() {
	os.Unsetenv(EnvExpensiveTests)
}
