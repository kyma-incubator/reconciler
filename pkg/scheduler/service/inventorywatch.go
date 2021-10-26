package service

import (
	"context"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"go.uber.org/zap"
)

type inventoryQueue chan<- *cluster.State

func newInventoryWatch(inventory cluster.Inventory, logger *zap.SugaredLogger, config *SchedulerConfig) *inventoryWatcher {
	return &inventoryWatcher{
		inventory: inventory,
		config:    config,
		logger:    logger,
	}
}

type inventoryWatcher struct {
	inventory cluster.Inventory
	config    *SchedulerConfig
	logger    *zap.SugaredLogger
}

func (w *inventoryWatcher) Inventory() cluster.Inventory {
	return w.inventory
}

func (w *inventoryWatcher) Run(ctx context.Context, queue inventoryQueue) error {
	w.logger.Infof("Starting inventory watcher with an watch-interval of %.1f secs",
		w.config.InventoryWatchInterval.Seconds())

	w.processClustersToReconcile(queue) //check for clusters now, otherwise first check would be trigger by ticker
	ticker := time.NewTicker(w.config.InventoryWatchInterval)
	for {
		select {
		case <-ticker.C:
			w.processClustersToReconcile(queue)
		case <-ctx.Done():
			w.logger.Info("Stopping inventory watcher because parent context got closed")
			ticker.Stop()
			return nil
		}
	}
}

func (w *inventoryWatcher) processClustersToReconcile(queue inventoryQueue) {
	clusterStates, err := w.inventory.ClustersToReconcile(w.config.ClusterReconcileInterval)
	if err != nil {
		w.logger.Errorf("Inventory watchers failed to fetch clusters to reconcile from inventory "+
			"(using reconcile interval of %.0f secs): %s",
			w.config.ClusterReconcileInterval.Seconds(), err)
		return
	}

	w.logger.Debugf("Inventory watcher found %d clusters which require a reconciliation", len(clusterStates))
	for _, clusterState := range clusterStates {
		if clusterState == nil {
			w.logger.Warn("Inventory watcher found nil cluster state when processing the list of clusters to reconcile")
			continue
		}
		w.logger.Infof("Inventory watcher added runtime '%s' to scheduling queue "+
			"(clusterVersion:%d/configVersion:%d/status:%s)",
			clusterState.Cluster.RuntimeID,
			clusterState.Cluster.Version, clusterState.Configuration.Version, clusterState.Status.Status)
		queue <- clusterState
	}
}
