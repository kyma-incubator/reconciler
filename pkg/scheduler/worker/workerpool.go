package worker

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/invoker"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
	"time"
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
	return w.run(ctx, false)
}

func (w *Pool) Run(ctx context.Context) error {
	return w.run(ctx, true)
}

func (w *Pool) run(ctx context.Context, checkWithInterval bool) error {
	workerPool, err := w.startWorkerPool(ctx)
	if err != nil {
		return err
	}
	defer func() {
		w.logger.Info("Stopping worker pool")
		workerPool.Release()
	}()

	if checkWithInterval {
		return w.invokeProcessableOpsWithInterval(ctx, workerPool)
	}
	return w.invokeProcessableOps(workerPool)
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
		w.logger.Errorf("Not able to assign operation '%s' to worker because state "+
			"of cluster '%s' could not be retrieved: %s", opEntity, opEntity.Cluster, err)
		return
	}

	w.logger.Debugf("Assigning operation '%s' to worker", opEntity)
	err = (&worker{
		reconRepo:  w.reconRepo,
		invoker:    w.invoker,
		logger:     w.logger,
		maxRetries: w.config.InvokerMaxRetries,
		retryDelay: w.config.InvokerRetryDelay,
	}).run(ctx, clusterState, opEntity)
	if err != nil {
		w.logger.Warnf("Worker assigned to operation '%s' returned an error: %s", opEntity, err)
	}
}

func (w *Pool) invokeProcessableOps(workerPool *ants.PoolWithFunc) error {
	w.logger.Debug("Checking for processable operations")
	ops, err := w.reconRepo.GetProcessableOperations()
	if err != nil {
		w.logger.Warnf("Failed to retrieve processable operations: %s", err)
		return err
	}

	foundOpsCnt := len(ops)
	w.logger.Debugf("Found %d processable operations", foundOpsCnt)
	if foundOpsCnt == 0 {
		return nil
	}

	for _, op := range ops {
		if err := workerPool.Invoke(op); err != nil {
			w.logger.Warnf("Failed to assign processable operation '%s' to a worker: %s", op, err)
			return err
		}
	}
	w.logger.Debugf("Assigned all %d processable operations to workers", foundOpsCnt)

	return nil
}

func (w *Pool) invokeProcessableOpsWithInterval(ctx context.Context, workerPool *ants.PoolWithFunc) error {
	w.logger.Debugf("Start checking for processable operations each %.1f seconds",
		w.config.OperationCheckInterval.Seconds())

	//check now otherwise first check would happen by ticker (after the configured interval is over)
	if err := w.invokeProcessableOps(workerPool); err != nil {
		return err
	}

	ticker := time.NewTicker(w.config.OperationCheckInterval)
	for {
		select {
		case <-ticker.C:
			if err := w.invokeProcessableOps(workerPool); err != nil {
				w.logger.Warnf("Failed to invoke all processable operations but will retry after %.1f seconds again",
					w.config.OperationCheckInterval.Seconds())
			}
		case <-ctx.Done():
			w.logger.Info("Stopping interval checks of processable operations because parent context got closed")
			ticker.Stop()
			return nil
		}
	}
}
