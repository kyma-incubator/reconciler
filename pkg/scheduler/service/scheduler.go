package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

const (
	defaultQueueSize                = 10
	defaultWatchInterval            = 1 * time.Minute
	defaultClusterReconcileInterval = 15 * time.Minute
)

type Config struct {
	WatchInterval            time.Duration
	ClusterReconcileInterval time.Duration
	ClusterQueueSize         int
}

func (wc *Config) validate() error {
	if wc.WatchInterval < 0 {
		return errors.New("watch interval cannot cannot be < 0")
	}
	if wc.WatchInterval == 0 {
		wc.WatchInterval = defaultWatchInterval
	}
	if wc.ClusterReconcileInterval < 0 {
		return errors.New("cluster reconciliation interval cannot cannot be < 0")
	}
	if wc.ClusterReconcileInterval == 0 {
		wc.ClusterReconcileInterval = defaultClusterReconcileInterval
	}
	if wc.ClusterQueueSize < 0 {
		return errors.New("cluster queue cannot be < 0")
	}
	if wc.ClusterQueueSize == 0 {
		wc.ClusterQueueSize = defaultQueueSize
	}
	return nil
}

type scheduler struct {
	logger        *zap.SugaredLogger
	preComponents []string
}

func newScheduler(preComponents []string, logger *zap.SugaredLogger) *scheduler {
	return &scheduler{
		preComponents: preComponents,
		logger:        logger,
	}
}

func (s *scheduler) RunOnce(clusterState *cluster.State, reconRepo reconciliation.Repository) error {
	_, err := reconRepo.CreateReconciliation(clusterState, s.preComponents)
	return err
}

func (s *scheduler) Run(ctx context.Context, transition *ClusterStatusTransition, config *Config) error {
	if err := config.validate(); err != nil {
		return err
	}

	queue := make(chan *cluster.State, config.ClusterQueueSize)
	s.startInventoryWatcher(ctx, transition.Inventory(), config, queue)

	for {
		select {
		case clusterState := <-queue:
			if err := transition.StartReconciliation(clusterState, s.preComponents); err != nil {
				s.logger.Warnf("Failed to start reconciliation for cluster '%s': %s", clusterState.Cluster.Cluster, err)
			}
		case <-ctx.Done():
			s.logger.Debug("Stopping remote scheduler because parent context got closed")
			return nil
		}
	}

}

func (s *scheduler) startInventoryWatcher(ctx context.Context, inventory cluster.Inventory, config *Config, queue chan *cluster.State) {
	s.logger.Infof("Starting inventory watcher")

	go func(ctx context.Context,
		clInv cluster.Inventory,
		logger *zap.SugaredLogger,
		queue chan *cluster.State,
		cfg *Config) {

		watcher, err := newInventoryWatch(clInv, logger, cfg)
		if err != nil {
			logger.Fatalf("Failed to start inventory watcher: %s", err)
		}
		if err := watcher.Run(ctx, queue); err != nil {
			logger.Errorf("Inventory watcher returned an error: %s", err)
		}

	}(ctx, inventory, s.logger, queue, config)
}
