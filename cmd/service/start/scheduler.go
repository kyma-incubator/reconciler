package cmd

import (
	"context"

	"github.com/kyma-incubator/reconciler/pkg/scheduler"
)

func startScheduler(ctx context.Context, o *Options) error {
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

	scheduler, err := scheduler.NewRemoteScheduler(inventoryWatch, o.Workers, o.Verbose)
	if err != nil {
		return err
	}

	return scheduler.Run(ctx)
}
