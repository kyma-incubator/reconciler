package scheduler

import (
	"context"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const defaultPoolSize = 50

type Scheduler interface {
	Run(ctx context.Context) error
}

type RemoteScheduler struct {
	inventoryWatch InventoryWatcher
	poolSize       int
	logger         *zap.Logger
}

func NewRemoteScheduler(inventoryWatch InventoryWatcher, workers int, debug bool) (Scheduler, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &RemoteScheduler{
		inventoryWatch: inventoryWatch,
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

	rs.logger.Debug("Starting worker pool with capacity %d workers")
	workersPool, err := ants.NewPoolWithFunc(rs.poolSize, func(i interface{}) {
		rs.Worker(i.(cluster.State))
	})
	if err != nil {
		return errors.Wrap(err, "failed to create worker pool of remote-scheduler")
	}

	go func(ctx context.Context, queue chan cluster.State) {
		if err := rs.inventoryWatch.Run(ctx, queue); err != nil {
			rs.logger.Error(fmt.Sprintf("Failed to run inventory watch: %s", err))
		}
	}(ctx, queue)

	for {
		select {
		case cluster := <-queue:
			go func(workersPool *ants.PoolWithFunc) {
				if err := workersPool.Invoke(cluster); err != nil {
					rs.logger.Error(fmt.Sprintf("Failed to pass cluster to cluster-pool worker: %s", err))
				}
			}(workersPool)
		case <-ctx.Done():
			rs.logger.Debug("Stopping remote scheduler because parent context got closed")
			return nil
		}
	}
}

func (rs *RemoteScheduler) Worker(cluster cluster.State) {

}

// func NewLocalScheduler() (Scheduler, error) {
// 	return &LocalScheduler{}, nil
// }

// type LocalScheduler struct{}

// func (ls *LocalScheduler) Run() {

// }
