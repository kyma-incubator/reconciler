package scheduler

import (
	"context"
	"fmt"

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
	workerFactory  *WorkersFactory
	poolSize       int
	logger         *zap.SugaredLogger
}

func NewRemoteScheduler(inventoryWatch InventoryWatcher, workerFactory *WorkersFactory, workers int, debug bool) (Scheduler, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &RemoteScheduler{
		inventoryWatch: inventoryWatch,
		workerFactory:  workerFactory,
		poolSize:       workers,
		logger:         logger,
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

	//Reconcile Cluster Essentials first to install CRDs
	for _, component := range components {
		if component.Component == "cluster-essentials" {
			worker, err := rs.workerFactory.ForComponent(component.Component)
			if err != nil {
				rs.logger.Errorf("Error creating worker for component: %s", err)
				continue
			}

			err = worker.Reconcile(component, state, schedulingID)
			if err != nil {
				rs.logger.Errorf("Error while reconciling component %s: %s", component.Component, err)
			}
		}
	}

	//Reconcile Istio
	for _, component := range components {
		if component.Component == "istio" {
			worker, err := rs.workerFactory.ForComponent(component.Component)
			if err != nil {
				rs.logger.Errorf("Error creating worker for component: %s", err)
				continue
			}

			err = worker.Reconcile(component, state, schedulingID)
			if err != nil {
				rs.logger.Errorf("Error while reconciling component %s: %s", component.Component, err)
			}
		}
	}

	//Reconcile the rest
	for _, component := range components {
		if component.Component != "istio" && component.Component != "cluster-essentials" {
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
}

// func NewLocalScheduler() (Scheduler, error) {
// 	return &LocalScheduler{}, nil
// }

// type LocalScheduler struct{}

// func (ls *LocalScheduler) Run() {

// }
