package scheduler

import (
	"context"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type concurrency bool

const (
	defaultPoolSize                   = 50
	concurrencyNotAllowed concurrency = false
	concurrencyAllowed    concurrency = true
)

type Scheduler interface {
	Run(ctx context.Context) error
}

type RemoteScheduler struct {
	inventoryWatch InventoryWatcher
	workerFactory  WorkerFactory
	mothershipCfg  MothershipReconcilerConfig
	poolSize       int
	logger         *zap.SugaredLogger
}

func NewRemoteScheduler(inventoryWatch InventoryWatcher, workerFactory WorkerFactory, mothershipCfg MothershipReconcilerConfig, workers int, debug bool) (Scheduler, error) {
	return &RemoteScheduler{
		inventoryWatch: inventoryWatch,
		workerFactory:  workerFactory,
		mothershipCfg:  mothershipCfg,
		poolSize:       workers,
		logger:         logger.NewLogger(debug),
	}, nil
}

func (rs *RemoteScheduler) validate() error {
	if rs.poolSize < 0 {
		return errors.New("worker pool size cannot be < 0")
	}
	if rs.poolSize == 0 {
		rs.poolSize = defaultPoolSize
	}
	return nil
}

func (rs *RemoteScheduler) Run(ctx context.Context) error {
	if err := rs.validate(); err != nil {
		return err
	}

	queue := make(chan cluster.State, rs.poolSize)

	rs.logger.Debugf("Starting worker pool with capacity %d workers", rs.poolSize)
	workersPool, err := ants.NewPoolWithFunc(rs.poolSize, func(i interface{}) {
		rs.schedule(ctx, i.(cluster.State))
	})
	if err != nil {
		return errors.Wrap(err, "failed to create worker pool of remote-scheduler")
	}

	go func(ctx context.Context, queue chan cluster.State) {
		if err := rs.inventoryWatch.Run(ctx, queue); err != nil {
			rs.logger.Errorf("Failed to run inventory watch: %s", err)
		}
	}(ctx, queue)

	for {
		select {
		case clusterState := <-queue:
			go func(workersPool *ants.PoolWithFunc) {
				if err := workersPool.Invoke(clusterState); err != nil {
					rs.logger.Errorf("Failed to pass cluster to cluster-pool worker: %s", err)
				}
			}(workersPool)
		case <-ctx.Done():
			rs.logger.Debug("Stopping remote scheduler because parent context got closed")
			return nil
		}
	}
}

func (rs *RemoteScheduler) schedule(ctx context.Context, state cluster.State) {
	schedulingID := uuid.NewString()
	components, err := state.Configuration.GetComponents(rs.mothershipCfg.PreComponents)
	if err != nil {
		rs.logger.Errorf("Failed to get components for cluster %s: %s", state.Cluster.Cluster, err)
		return
	}

	if components == nil {
		rs.logger.Infof("No components to reconcile for cluster %s", state.Cluster.Cluster)
		return
	}

	statusUpdater := NewClusterStatusUpdater(rs.inventoryWatch.Inventory(), state, components, rs.logger)
	go statusUpdater.Run(ctx)

	handler := NewReconciliationHandler(rs.workerFactory).WithStatusUpdater(statusUpdater)
	err = handler.Reconcile(ctx, components, &state, schedulingID)
	if err != nil {
		rs.logger.Errorf("Failed to reconcile components for cluster %s: %s", state.Cluster.Cluster, err)
		return
	}
}
