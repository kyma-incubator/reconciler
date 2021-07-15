package cmd

import (
	"context"

	"github.com/kyma-incubator/reconciler/pkg/scheduler"
)

func startScheduler(o *Options, ctx context.Context) error {
	inventoryWatch, err := scheduler.NewInventoryWatch(
		o.Registry.Inventory(),
		o.Registry.Logger())
	if err != nil {
		return err
	}

	scheduler, err := scheduler.NewRemoteScheduler(inventoryWatch)
	if err != nil {
		return err
	}

	return scheduler.Run(ctx)
}
