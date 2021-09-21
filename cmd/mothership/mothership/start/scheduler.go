package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/kyma-incubator/reconciler/pkg/scheduler"
	"github.com/spf13/viper"
)

func startScheduler(ctx context.Context, o *Options, configFile string) error {
	mothershipCfg, err := parseMothershipReconcilerConfig(configFile)
	if err != nil {
		return err
	}

	reconcilersCfg, err := parseComponentReconcilersConfig(o.ReconcilersCfgPath)
	if err != nil {
		return err
	}

	inventoryWatch, err := scheduler.NewInventoryWatch(
		o.Registry.Inventory(),
		o.Verbose,
		&scheduler.InventoryWatchConfig{
			WatchInterval:            o.WatchInterval,
			ClusterReconcileInterval: o.ClusterReconcileInterval,
		},
	)
	if err != nil {
		return err
	}

	workerFactory, err := scheduler.NewRemoteWorkerFactory(
		o.Registry.Inventory(),
		reconcilersCfg,
		mothershipCfg,
		o.Registry.OperationsRegistry(),
		o.Verbose,
	)
	if err != nil {
		return err
	}

	remoteScheduler, err := scheduler.NewRemoteScheduler(
		inventoryWatch,
		workerFactory,
		mothershipCfg,
		o.Workers,
		o.Verbose,
	)
	if err != nil {
		return err
	}

	return remoteScheduler.Run(ctx)
}

func parseMothershipReconcilerConfig(configFile string) (scheduler.MothershipReconcilerConfig, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return scheduler.MothershipReconcilerConfig{}, err
	}

	return scheduler.MothershipReconcilerConfig{
		Scheme:        viper.GetString("mothership.scheme"),
		Host:          viper.GetString("mothership.host"),
		Port:          viper.GetInt("mothership.port"),
		PreComponents: viper.GetStringSlice("preComponents")}, nil
}

func parseComponentReconcilersConfig(path string) (scheduler.ComponentReconcilersConfig, error) {
	serialized, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error while reading component reconcilers configuration: %s", err)
	}

	var config scheduler.ComponentReconcilersConfig
	err = json.Unmarshal(serialized, &config)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshaling component reconcilers configuration: %s", err)
	}

	return config, nil
}
