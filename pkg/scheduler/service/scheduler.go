package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
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

type Scheduler struct {
	reconRepo     reconciliation.Repository
	logger        *zap.SugaredLogger
	preComponents []string
}

func NewScheduler(reconRepo reconciliation.Repository, preComponents []string, debug bool) *Scheduler {
	return &Scheduler{
		reconRepo:     reconRepo,
		preComponents: preComponents,
		logger:        log.NewLogger(debug),
	}
}

func (s *Scheduler) RunOnce(clusterState *cluster.State) error {
	_, err := s.reconRepo.CreateReconciliation(clusterState, s.preComponents)
	return err
}

func (s *Scheduler) Run(ctx context.Context, inventory cluster.Inventory, config *Config) error {
	if err := config.validate(); err != nil {
		return err
	}

	queue := make(chan *cluster.State, config.ClusterQueueSize)
	s.startInventoryWatcher(ctx, inventory, config, queue)

	for {
		select {
		case clusterState := <-queue:
			_, err := s.reconRepo.CreateReconciliation(clusterState, s.preComponents)
			if err != nil {
				if reconciliation.IsDuplicateClusterReconciliationError(err) {
					s.logger.Infof("Tried to add cluster '%s' to reconciliation queue but "+
						"cluster is already enqueued", clusterState.Cluster.Cluster)
				} else {
					s.logger.Errorf("Failed to add cluster '%s' to reconciliation queue: %s",
						clusterState.Cluster.Cluster, err)
					break
				}
			}
		case <-ctx.Done():
			s.logger.Debug("Stopping remote scheduler because parent context got closed")
			return nil
		}
	}

}

func (s *Scheduler) startInventoryWatcher(ctx context.Context, inventory cluster.Inventory, config *Config, queue chan *cluster.State) {
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
