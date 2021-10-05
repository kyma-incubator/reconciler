package worker

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/invoker"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"go.uber.org/zap"
	"strings"
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
	if !w.isProcessable(op) {
		w.logger.Warnf("Worker cannot start processing of operation '%s' because it is in non-processable state '%s'",
			op, op.State)
		return nil
	}

	w.logger.Debugf("Worker starts processing of operation '%s'", op)

	//mark operation to be now in progress (this avoids that it will be picked up by another worker)
	if err := w.updateOperationState(op, model.OperationStateInProgress); err != nil {
		return err
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
		w.logger.Debugf("Worker calls invoker for operation '%s' (in retryable function)", op)
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

	if err == nil {
		w.logger.Debugf("Worker finished processing of operation '%s' successfully", op)
	} else {
		w.logger.Warnf("Worker stops processing operation '%s' because invoker "+
			"returned consistently errors (%d retries): %s", op, w.maxRetries, err)
	}
	return err
}

func (w *worker) updateOperationState(op *model.OperationEntity, state model.OperationState, reasons ...string) error {
	reason := strings.Join(reasons, ", ")
	if err := w.reconRepo.UpdateOperationState(op.SchedulingID, op.CorrelationID, state, reason); err != nil {
		return fmt.Errorf("worker failed to update operation '%s' to state '%s': %s", op, state, err)
	}
	return nil
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
