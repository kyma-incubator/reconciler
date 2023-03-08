package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	defaultQueueSize                = 50
	defaultInventoryWatchInterval   = 1 * time.Minute
	defaultClusterReconcileInterval = 15 * time.Minute

	DeleteStrategySystem DeleteStrategy = "system"
	DeleteStrategyAll    DeleteStrategy = "all"
)

type DeleteStrategy string

func NewDeleteStrategy(s string) (DeleteStrategy, error) {
	switch strings.ToLower(s) {
	case "": // return default if empty
		return DeleteStrategySystem, nil
	case string(DeleteStrategySystem):
		return DeleteStrategySystem, nil
	case string(DeleteStrategyAll):
		return DeleteStrategyAll, nil
	default:
		return "", errors.Errorf("Delete strategy %s not supported", s)
	}
}

type SchedulerConfig struct {
	PreComponents            [][]string
	InventoryWatchInterval   time.Duration
	ClusterReconcileInterval time.Duration
	ClusterQueueSize         int
	DeleteStrategy           DeleteStrategy
	ComponentCRDs            map[string]config.ComponentCRD
}

func (wc *SchedulerConfig) validate() error {
	if wc.InventoryWatchInterval < 0 {
		return errors.New("inventory watch interval cannot be < 0")
	}
	if wc.InventoryWatchInterval == 0 {
		wc.InventoryWatchInterval = defaultInventoryWatchInterval
	}
	if wc.ClusterReconcileInterval < 0 {
		return errors.New("cluster reconciliation interval cannot be < 0")
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
	switch wc.DeleteStrategy {
	case "": // set default if empty (should not happen)
		wc.DeleteStrategy = DeleteStrategySystem
	case DeleteStrategyAll, DeleteStrategySystem: // valid options
		break
	default: // invalid
		return errors.Errorf("Delete strategy %s not supported", wc.DeleteStrategy)
	}
	return nil
}

type scheduler struct {
	logger *zap.SugaredLogger
}

func newScheduler(logger *zap.SugaredLogger) *scheduler {
	return &scheduler{
		logger: logger,
	}
}

func (s *scheduler) RunOnce(clusterState *cluster.State, reconRepo reconciliation.Repository, config *SchedulerConfig) error {
	s.logger.Debugf("Starting local scheduler")
	reconEntity, err := reconRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{
		PreComponents:        config.PreComponents,
		DeleteStrategy:       string(config.DeleteStrategy),
		ReconciliationStatus: clusterState.Status.Status,
	})
	if err == nil {
		s.logger.Debugf("Scheduler created reconciliation entity: '%s", reconEntity)
	}
	return err
}

func (s *scheduler) Run(ctx context.Context, transition *ClusterStatusTransition, config *SchedulerConfig) error {
	if err := config.validate(); err != nil {
		return err
	}

	queue := make(chan *cluster.State, config.ClusterQueueSize)
	s.startInventoryWatcher(ctx, transition.Inventory(), config, queue)

	for {
		select {
		case clusterState := <-queue:
			if err := transition.StartReconciliation(clusterState.Cluster.RuntimeID, clusterState.Configuration.Version, config); err == nil {
				s.logger.Debugf("Scheduler triggered reconciliation for cluster '%s' "+
					"(clusterVersion:%d/configVersion:%d/status:%s/last status update:%.2f min)", clusterState.Cluster.RuntimeID,
					clusterState.Cluster.Version, clusterState.Configuration.Version, clusterState.Status.Status,
					time.Since(clusterState.Status.Created).Minutes())
			} else {
				s.logger.Warn(err)
			}
		case <-ctx.Done():
			s.logger.Debug("Stopping remote scheduler because parent context got closed")
			return nil
		}
	}

}

func (s *scheduler) startInventoryWatcher(ctx context.Context, inventory cluster.Inventory, config *SchedulerConfig, queue chan *cluster.State) {
	s.logger.Infof("Starting inventory watcher")

	go func(ctx context.Context,
		clInv cluster.Inventory,
		logger *zap.SugaredLogger,
		queue chan *cluster.State,
		cfg *SchedulerConfig) {

		watcher := newInventoryWatch(clInv, logger, cfg)
		if err := watcher.Run(ctx, queue); err != nil {
			logger.Errorf("Inventory watcher returned an error: %s", err)
		}

	}(ctx, inventory, s.logger, queue, config)
}
