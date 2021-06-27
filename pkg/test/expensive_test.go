package test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpensive(t *testing.T) {
	require.False(t, RunExpensiveTests())

	os.Setenv(EnvExpensiveTests, "false")
	require.False(t, RunExpensiveTests())

	os.Setenv(EnvExpensiveTests, "0")
	require.False(t, RunExpensiveTests())

	os.Setenv(EnvExpensiveTests, "foo")
	require.False(t, RunExpensiveTests())

	os.Setenv(EnvExpensiveTests, "trUe")
	require.True(t, RunExpensiveTests())

	os.Setenv(EnvExpensiveTests, "1")
	require.True(t, RunExpensiveTests())

	DisableExpensiveTests()
	require.False(t, RunExpensiveTests())

	EnableExpensiveTests()
	require.True(t, RunExpensiveTests())
}
