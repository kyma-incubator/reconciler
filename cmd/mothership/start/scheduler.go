package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
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

func parseMothershipReconcilerConfig(configFile string) (reconciler.MothershipReconcilerConfig, error) {
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return reconciler.MothershipReconcilerConfig{}, err
	}
	mothershipHost := viper.GetString("mothership.host")
	mothershipPort := viper.GetInt("mothership.port")
	crdComponents := viper.GetStringSlice("crdComponents")
	preComponents := viper.GetStringSlice("preComponents")
	return reconciler.MothershipReconcilerConfig{
		Host:          mothershipHost,
		Port:          mothershipPort,
		CrdComponents: crdComponents,
		PreComponents: preComponents}, nil
}

func parseComponentReconcilersConfig(path string) (reconciler.ComponentReconcilersConfig, error) {
	serialized, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error while reading component reconcilers configuration: %s", err)
	}

	var config reconciler.ComponentReconcilersConfig
	err = json.Unmarshal(serialized, &config)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshaling component reconcilers configuration: %s", err)
	}

	return config, nil
}
