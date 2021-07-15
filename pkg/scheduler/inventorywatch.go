package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"go.uber.org/zap"
)

type InventoryQueue chan<- cluster.State

type InventoryWatcher interface {
	Run(ctx context.Context, informer InventoryQueue)
}

func NewInventoryWatch(inventory cluster.Inventory, logger *zap.Logger) (InventoryWatcher, error) {
	return &DefaultInventoryWatcher{inventory, logger}, nil
}

type DefaultInventoryWatcher struct {
	inventory cluster.Inventory
	logger    *zap.Logger
}

func (w *DefaultInventoryWatcher) Run(ctx context.Context, informer InventoryQueue) {
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		default:
			clusters, err := w.inventory.ClustersToReconcile(1 * time.Minute)
			if err != nil {
				w.logger.Error(fmt.Sprintf("Error while fetching clusters to reconcile from inventory: %s", err))
			}

			if len(clusters) == 0 {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			for _, cluster := range clusters {
				if cluster == nil {
					w.logger.Debug("Nil cluster state when processing clusters to reconcile list")
					continue
				}
				informer <- *cluster
			}
		}
	}
}
