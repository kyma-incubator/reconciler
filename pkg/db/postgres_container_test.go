package db

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPostgresContainer(t *testing.T) {
	test.IntegrationTest(t)
	ctx := context.Background()

	configFile, err := test.GetConfigFile()
	require.NoError(t, err)

	viper.SetConfigFile(configFile)

	configReadError := viper.ReadInConfig()
	require.NoError(t, configReadError)

	env := getPostgresEnvironment()

	t.Run("Run New Postgres Container", func(t *testing.T) {
		t.Parallel()
		c := NewPostgresContainer(env)

		require.NoError(t, c.Bootstrap(ctx), "container should start normally")

		t.Cleanup(func() {
			require.NoError(t, c.Terminate(ctx))
		})

		stateAfterStart, startStateFetchError := c.State(ctx)
		require.NoError(t, startStateFetchError, "container should have stateAfterStart")

		require.True(t, stateAfterStart.Running, "container should be running")
	})
}
