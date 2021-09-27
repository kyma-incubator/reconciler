package cmd

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/service"
	"github.com/spf13/viper"
)

func startScheduler(ctx context.Context, o *Options, configFile string) error {
	schedulerCfg, err := parseSchedulerConfig(configFile)
	if err != nil {
		return err
	}

	runtimeBuilder := service.NewRuntimeBuilder(o.Registry.ReconciliationRepository(), logger.NewLogger(o.Verbose))

	runtimeBuilder.
		WithSchedulerConfig(
			&service.SchedulerConfig{
				InventoryWatchInterval:   o.WatchInterval,
				ClusterReconcileInterval: o.ClusterReconcileInterval,
				ClusterQueueSize:         o.Workers,
			}).
		RunRemote(
			o.Registry.Connnection(),
			o.Registry.Inventory(),
			schedulerCfg).
		Run(ctx)

	return nil
}

func parseSchedulerConfig(configFile string) (*config.Config, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return &config.Config{}, err
	}

	var cfg *config.Config
	return cfg, viper.UnmarshalKey("mothership", cfg)
}
