package config

import (
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig(t *testing.T) {
	cfgFile, err := test.GetConfigFile()
	require.NoError(t, err)

	cfg := &Config{}

	viper.SetConfigFile(cfgFile)
	require.NoError(t, viper.ReadInConfig())

	require.NoError(t, viper.UnmarshalKey("mothership", cfg))
	require.NotEmpty(t, cfg.Scheduler.Reconcilers[FallbackComponentReconciler])
}
