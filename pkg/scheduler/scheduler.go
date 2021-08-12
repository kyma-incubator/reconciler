package scheduler

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	defaultPoolSize = 50
)

type Scheduler interface {
	Run(ctx context.Context) error
}

type RemoteScheduler struct {
	inventoryWatch InventoryWatcher
	workerFactory  WorkerFactory
	poolSize       int
	logger         *zap.SugaredLogger
}

func NewRemoteScheduler(inventoryWatch InventoryWatcher, workerFactory WorkerFactory, workers int, debug bool) (Scheduler, error) {
	l, err := logger.NewLogger(false)
	if err != nil {
		return nil, err
	}
	return &RemoteScheduler{
		inventoryWatch: inventoryWatch,
		workerFactory:  workerFactory,
		poolSize:       workers,
		logger:         l,
	}, nil
}

func (rs *RemoteScheduler) validate() error {
	if rs.poolSize < 0 {
		return fmt.Errorf("Worker pool size cannot be < 0")
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
		rs.schedule(i.(cluster.State))
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
		case cluster := <-queue:
			go func(workersPool *ants.PoolWithFunc) {
				if err := workersPool.Invoke(cluster); err != nil {
					rs.logger.Errorf("Failed to pass cluster to cluster-pool worker: %s", err)
				}
			}(workersPool)
		case <-ctx.Done():
			rs.logger.Debug("Stopping remote scheduler because parent context got closed")
			return nil
		}
	}
}

func (rs *RemoteScheduler) schedule(state cluster.State) {
	schedulingID := uuid.NewString()
	components, err := state.Configuration.GetComponents()
	if err != nil {
		rs.logger.Errorf("Failed to get components for cluster %s: %s", state.Cluster.Cluster, err)
		return
	}

	if len(components) == 0 {
		rs.logger.Infof("No components to reconcile for cluster %s", state.Cluster.Cluster)
		return
	}

	for _, component := range components {
		worker, err := rs.workerFactory.ForComponent(component.Component)
		if err != nil {
			rs.logger.Errorf("Error creating worker for component: %s", err)
			continue
		}
		go func(component *keb.Components, state cluster.State, schedulingID string) {
			err := worker.Reconcile(component, state, schedulingID)
			if err != nil {
				rs.logger.Errorf("Error while reconciling component %s: %s", component.Component, err)
			}
		}(component, state, schedulingID)
	}
}

type LocalScheduler struct {
	clusterState  cluster.State
	workerFactory WorkerFactory
	logger        *zap.SugaredLogger
}

func NewLocalScheduler(cs cluster.State) (Scheduler, error) {
	l, err := logger.NewLogger(false)
	if err != nil {
		return nil, err
	}
	return &LocalScheduler{
		clusterState: cs,
		logger:       l,
	}, nil
}

func (ls *LocalScheduler) Run(ctx context.Context) error {
	schedulingID := uuid.NewString()

	components, err := ls.clusterState.Configuration.GetComponents()
	if err != nil {
		return fmt.Errorf("failed to get components: %s", err)
	}

	var wg sync.WaitGroup
	wg.Add(len(components))

	for _, component := range components {
		worker, err := ls.workerFactory.ForComponent(component.Component)
		if err != nil {
			return fmt.Errorf("failed to create a: %s", err)
		}

		go func(component *keb.Components, state cluster.State, schedulingID string) {
			defer wg.Done()
			err := worker.Reconcile(component, state, schedulingID)
			if err != nil {
				ls.logger.Errorf("Error while reconciling component %s: %s", component.Component, err)
			}
		}(component, ls.clusterState, schedulingID)
	}

	wg.Wait()

	return nil
}
