package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
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
	antsPool  *ants.PoolWithFunc
}

func NewWorkerPool(retriever ClusterStateRetriever, reconRepo reconciliation.Repository, invoker invoker.Invoker, config *Config, logger *zap.SugaredLogger) (*Pool, error) {

	if config == nil {
		config = &Config{}
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	return &Pool{
		retriever: retriever,
		reconRepo: reconRepo,
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
		if err != nil {
			w.logger.Errorf("Unable to remove worker pool occupancy: %v", err)
		}
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
		if repository.IsNotFoundError(err) { // discard the orphaned operation, it will never succeed if the cluster is gone
			discardMsg := fmt.Sprintf("Operation '%s' belongs to a no longer existing cluster (%s) and will be discarded", opEntity, opEntity.RuntimeID)
			w.logger.Warn(discardMsg)

			if err := w.reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID, model.OperationStateError, false, discardMsg); err != nil {
				w.logger.Errorf("Error updating state of orphaned operation '%s': %s", opEntity, err)
			}
		} else {
			w.logger.Errorf("Worker pool is not able to assign operation '%s' to worker because state "+
				"of cluster '%s' could not be retrieved: %s", opEntity, opEntity.RuntimeID, err)
		}
		return
	}

	w.logger.Debugf("Worker pool is assigning operation '%s' to worker", opEntity)
	maxOpRetries := w.config.MaxOperationRetries - int(opEntity.Retries)
	err = (&worker{
		reconRepo:  w.reconRepo,
		invoker:    w.invoker,
		logger:     w.logger,
		maxRetries: w.config.InvokerMaxRetries,
		retryDelay: w.config.InvokerRetryDelay,
	}).run(ctx, clusterState, opEntity, maxOpRetries)
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

	ops = w.filterProcessableOpsByMaxRetries(ops)
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

	idx := 0
	for idx < opsCnt {
		if workerPool.Free() == 0 {
			remainingOpsCnt := opsCnt - idx
			w.logger.Warnf("could not assign %d operations to workers because workerpool capacity reached: capacity=%d", remainingOpsCnt, workerPool.Cap())
			break
		}
		op := ops[idx]
		if err := workerPool.Invoke(op); err == nil {
			w.logger.Infof("Worker pool assigned worker to reconcile component '%s' on cluster '%s' (%s)",
				op.Component, op.RuntimeID, op)
		} else {
			w.logger.Warnf("Worker pool failed to assign worker to operation '%s': %s", op, err)
			return idx + 1, err
		}
		idx++
	}
	w.logger.Infof("Worker pool assigned %d of %d processable operations to workers", idx, opsCnt)
	return opsCnt, nil
}

func (w *Pool) filterProcessableOpsByMaxRetries(ops []*model.OperationEntity) []*model.OperationEntity {
	var filteredOps []*model.OperationEntity
	for _, op := range ops {
		if op.Retries >= int64(w.config.MaxOperationRetries) {
			err := w.reconRepo.UpdateOperationState(op.SchedulingID, op.CorrelationID, model.OperationStateError, true, fmt.Sprintf("operation exceeds max. operation retries limit (maxOperationRetries:%d)", w.config.MaxOperationRetries))
			if err != nil {
				w.logger.Warnf("could not update operation state with schedulingID %s and correlationID %s to %v state", op.SchedulingID, op.CorrelationID, model.OperationStateError)
			}
		} else {
			filteredOps = append(filteredOps, op)
		}
	}
	return filteredOps
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

func (w *Pool) IsClosed() bool {
	if w.antsPool == nil {
		return true
	}
	return w.antsPool.IsClosed()
}

func (w *Pool) RunningWorkers() (int, error) {
	if w.antsPool == nil {
		return 0, fmt.Errorf("could not retrieve number of running workers: worker pool is nil")
	}
	return w.antsPool.Running(), nil
}

func (w *Pool) Size() (int, error) {
	if w.antsPool == nil {
		return 0, fmt.Errorf("could not retrieve pool size: worker pool is nil")
	}
	return w.antsPool.Cap(), nil
}
