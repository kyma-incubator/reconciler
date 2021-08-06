package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/scheduler"
)

func startScheduler(ctx context.Context, o *Options) error {
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

	workerFactory, err := scheduler.NewWorkersFactory(
		o.Registry.Inventory(),
		reconcilersCfg,
		o.Registry.OperationsRegistry(),
		o.Verbose,
	)
	if err != nil {
		return err
	}

	scheduler, err := scheduler.NewRemoteScheduler(
		inventoryWatch,
		workerFactory,
		o.Workers,
		o.Verbose,
	)
	if err != nil {
		return err
	}

	return scheduler.Run(ctx)
}

func parseComponentReconcilersConfig(path string) (reconciler.ComponentReconcilersConfig, error) {
	serialized, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Error while reading component reconcilers configuration: %s", err)
	}

	var config reconciler.ComponentReconcilersConfig
	err = json.Unmarshal(serialized, &config)
	if err != nil {
		return nil, fmt.Errorf("Error while unmarshaling component reconcilers configuration: %s", err)
	}

	return config, nil
}
