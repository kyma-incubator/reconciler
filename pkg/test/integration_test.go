package test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntegration(t *testing.T) {
	require.NoError(t, os.Setenv(EnvIntegrationTests, "false"))
	require.False(t, IsIntegrationTestEnabled())

	require.NoError(t, os.Setenv(EnvIntegrationTests, "0"))
	require.False(t, IsIntegrationTestEnabled())

	require.NoError(t, os.Setenv(EnvIntegrationTests, "foo"))
	require.False(t, IsIntegrationTestEnabled())

	require.NoError(t, os.Setenv(EnvIntegrationTests, "trUe"))
	require.True(t, IsIntegrationTestEnabled())

	require.NoError(t, os.Setenv(EnvIntegrationTests, "1"))
	require.True(t, IsIntegrationTestEnabled())

	require.NoError(t, DisableIntegrationTests())
	require.False(t, IsIntegrationTestEnabled())

	require.NoError(t, EnableIntegrationTests())
	require.True(t, IsIntegrationTestEnabled())
}
