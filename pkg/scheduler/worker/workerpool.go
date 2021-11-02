package worker

import (
	"context"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/invoker"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Pool struct {
	retriever ClusterStateRetriever
	reconRepo reconciliation.Repository
	invoker   invoker.Invoker
	config    *Config
	logger    *zap.SugaredLogger
}

func NewWorkerPool(
	retriever ClusterStateRetriever,
	repo reconciliation.Repository,
	invoker invoker.Invoker,
	config *Config,
	logger *zap.SugaredLogger) (*Pool, error) {

	if config == nil {
		config = &Config{}
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	return &Pool{
		retriever: retriever,
		reconRepo: repo,
		invoker:   invoker,
		config:    config,
		logger:    logger,
	}, nil
}

func (w *Pool) RunOnce(ctx context.Context) error {
	return w.run(ctx, true)
}

func (w *Pool) Run(ctx context.Context) error {
	return w.run(ctx, false)
}

func (w *Pool) run(ctx context.Context, runOnce bool) error {
	workerPool, err := w.startWorkerPool(ctx)
	if err != nil {
		return err
	}
	defer func() {
		w.logger.Info("Stopping worker pool")
		workerPool.Release()
	}()

	if runOnce {
		return w.invokeProcessableOpsOnce(ctx, workerPool)
	}
	return w.invokeProcessableOpsWithInterval(ctx, workerPool)
}

func (w *Pool) invokeProcessableOpsOnce(ctx context.Context, workerPool *ants.PoolWithFunc) error {
	//wait until workers are ready
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			runningWorkers := workerPool.Running()
			if runningWorkers == 0 {
				w.logger.Debug("Worker pool has no running workers")

				invoked, err := w.invokeProcessableOps(workerPool)
				if err != nil {
					return errors.Wrap(err, "worker pool failed to assign processable operations to workers")
				}
				if invoked == 0 {
					w.logger.Debug("Worker pool invoked all processable operations")
					return nil
				}
			} else {
				w.logger.Debugf("Worker pool is waiting for %d workers to be finished", runningWorkers)
			}
		case <-ctx.Done():
			w.logger.Info("Stopping worker pool because parent context got closed")
			workerPool.Release()
			return nil
		}
	}
}

func (w *Pool) startWorkerPool(ctx context.Context) (*ants.PoolWithFunc, error) {
	w.logger.Infof("Starting worker pool with capacity of %d workers", w.config.PoolSize)
	return ants.NewPoolWithFunc(w.config.PoolSize, func(op interface{}) {
		w.assignWorker(ctx, op.(*model.OperationEntity))
	})
}

func (w *Pool) assignWorker(ctx context.Context, opEntity *model.OperationEntity) {
	clusterState, err := w.retriever.Get(opEntity)
	if err != nil {
		w.logger.Errorf("Worker pool is not able to assign operation '%s' to worker because state "+
			"of cluster '%s' could not be retrieved: %s", opEntity, opEntity.RuntimeID, err)
		return
	}

	w.logger.Debugf("Worker pool is assigning operation '%s' to worker", opEntity)
	err = (&worker{
		reconRepo:  w.reconRepo,
		invoker:    w.invoker,
		logger:     w.logger,
		maxRetries: w.config.InvokerMaxRetries,
		retryDelay: w.config.InvokerRetryDelay,
	}).run(ctx, clusterState, opEntity)
	if err != nil {
		w.logger.Warnf("Worker pool received an error from worker assigned to operation '%s': %s", opEntity, err)
	}
}

func (w *Pool) invokeProcessableOps(workerPool *ants.PoolWithFunc) (int, error) {
	w.logger.Debugf("Worker pool is checking for processable operations (max parallel ops per cluster: %d)",
		w.config.MaxParallelOperations)
	ops, err := w.reconRepo.GetProcessableOperations(w.config.MaxParallelOperations)
	if err != nil {
		w.logger.Warnf("Worker pool failed to retrieve processable operations: %s", err)
		return 0, err
	}

	opsCnt := len(ops)
	w.logger.Debugf("Worker pool found %d processable operations: %s", opsCnt, func() string {
		var opNames []string
		for _, op := range ops {
			opNames = append(opNames, op.Component)
		}
		return strings.Join(opNames, ", ")
	}())
	if opsCnt == 0 {
		return 0, nil
	}

	for idx, op := range ops {
		if err := workerPool.Invoke(op); err == nil {
			w.logger.Infof("Worker pool assigned worker to reconcile component '%s' on cluster '%s' (%s)",
				op.Component, op.RuntimeID, op)
		} else {
			w.logger.Warnf("Worker pool failed to assign worker to operation '%s': %s", op, err)
			return idx + 1, err
		}
	}
	w.logger.Debugf("Worker pool assigned %d processable operations to workers", opsCnt)

	return opsCnt, nil
}

func (w *Pool) invokeProcessableOpsWithInterval(ctx context.Context, workerPool *ants.PoolWithFunc) error {
	w.logger.Debugf("Worker pool starts watching for processable operations each %.1f secs",
		w.config.OperationCheckInterval.Seconds())

	//check now otherwise first check would happen by ticker (after the configured interval is over)
	if _, err := w.invokeProcessableOps(workerPool); err != nil {
		return err
	}

	ticker := time.NewTicker(w.config.OperationCheckInterval)
	for {
		select {
		case <-ticker.C:
			if _, err := w.invokeProcessableOps(workerPool); err != nil {
				w.logger.Warnf("Worker pool failed to invoke all processable operations "+
					"but will retry after %.1f seconds again",
					w.config.OperationCheckInterval.Seconds())
			}
		case <-ctx.Done():
			w.logger.Info("Worker pool is stopping interval checks of processable operations " +
				"because parent context got closed")
			ticker.Stop()
			return nil
		}
	}
}
