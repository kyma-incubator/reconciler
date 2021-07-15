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
	Run(ctx context.Context, informer InventoryQueue) error
}

func NewInventoryWatch(inventory cluster.Inventory, logger *zap.Logger) (InventoryWatcher, error) {
	return &DefaultInventoryWatcher{inventory, logger}, nil
}

type DefaultInventoryWatcher struct {
	inventory cluster.Inventory
	logger    *zap.Logger
}

func (w *DefaultInventoryWatcher) Run(ctx context.Context, queue InventoryQueue) error {
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		default:
			w.processClustersToReconcile(queue)
		}
	}

	return nil
}

func (w *DefaultInventoryWatcher) processClustersToReconcile(queue InventoryQueue) {
	clusters, err := w.inventory.ClustersToReconcile(1 * time.Minute)
	if err != nil {
		w.logger.Error(fmt.Sprintf("Error while fetching clusters to reconcile from inventory: %s", err))
		return
	}

	if len(clusters) == 0 {
		time.Sleep(500 * time.Millisecond)
		return
	}

	for _, cluster := range clusters {
		if cluster == nil {
			w.logger.Debug("Nil cluster state when processing the list of clusters to reconcile")
			continue
		}
		queue <- *cluster
	}
}
