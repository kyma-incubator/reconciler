package worker

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/invoker"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

type worker struct {
	reconRepo  reconciliation.Repository
	invoker    invoker.Invoker
	logger     *zap.SugaredLogger
	maxRetries int
	retryDelay time.Duration
}

func (w *worker) run(ctx context.Context, clusterState *cluster.State, op *model.OperationEntity) error {
	w.logger.Debugf("Start processing of operation '%s'", op)
	if w.isProcessable(op) {
		//mark operation to be now in progress
		err := w.reconRepo.UpdateOperationState(op.SchedulingID, op.CorrelationID, model.OperationStateInProgress)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("worker failed to update state of operation "+
				"(schedulingID:%s/correlationID:%s) to '%s'",
				op.SchedulingID, op.CorrelationID, model.OperationStateInProgress))
		}

		compsReady, err := w.componentsReady(op)
		if err != nil {
			return err
		}

		comp, err := clusterState.Configuration.GetComponent(op.Component)
		if err != nil {
			return err
		}

		retryable := func() error {
			return w.invoker.Invoke(ctx, &invoker.Params{
				ComponentToReconcile: comp,
				ComponentsReady:      compsReady,
				SchedulingID:         op.SchedulingID,
				CorrelationID:        op.CorrelationID,
				ClusterState:         clusterState,
			})
		}

		//retry calling the invoker if error was returned
		err = retry.Do(retryable,
			retry.Attempts(uint(w.maxRetries)),
			retry.Delay(w.retryDelay),
			retry.LastErrorOnly(false),
			retry.Context(ctx))

		if err != nil {
			w.logger.Warnf("Giving up processing of operation '%s' because worker retrieved consistenly errors "+
				"when calling invoker: %s", op, err)
		}
		return err
	} else {
		w.logger.Warnf("Worker stopped processing of operation '%s' because operation is in non-processable state '%s'",
			op, op.State)
		return nil
	}
}

func (w *worker) componentsReady(op *model.OperationEntity) ([]string, error) {
	opsReady, err := w.reconRepo.GetOperations(op.SchedulingID, model.OperationStateDone)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, opReady := range opsReady {
		result = append(result, opReady.Component)
	}
	return result, nil
}

func (w *worker) isProcessable(op *model.OperationEntity) bool {
	return op.State != model.OperationStateDone &&
		op.State != model.OperationStateError &&
		op.State != model.OperationStateInProgress
}
