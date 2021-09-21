package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"go.uber.org/zap"
)

const (
	DefaultReconciler = "base" //TODO: take this information configurable
	MaxRetryCount     = 20
	MaxDuration       = time.Hour
)

type ReconciliationWorker interface {
	Reconcile(ctx context.Context, component *keb.Component, state cluster.State, schedulingID string) error
}

type Worker struct {
	correlationID string
	config        *ComponentReconciler
	inventory     cluster.Inventory
	operationsReg OperationsRegistry
	invoker       reconcilerInvoker
	logger        *zap.SugaredLogger
	errorsCount   int
}

func NewWorker(
	config *ComponentReconciler,
	inventory cluster.Inventory,
	operationsReg OperationsRegistry,
	invoker reconcilerInvoker,
	logger *zap.SugaredLogger) (*Worker, error) {
	return &Worker{
		correlationID: uuid.NewString(),
		config:        config,
		inventory:     inventory,
		operationsReg: operationsReg,
		invoker:       invoker,
		logger:        logger,
		errorsCount:   0,
	}, nil
}

func (w *Worker) Reconcile(ctx context.Context, component *keb.Component, state cluster.State, schedulingID string) error {
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-time.After(MaxDuration):
			return fmt.Errorf("max operation time reached for operation %s in %s", w.correlationID, schedulingID)
		case <-ticker.C:
			done, err := w.process(ctx, component, state, schedulingID)
			if err != nil {
				// At this point something critical happened, we need to give up
				return err
			}
			if done {
				return nil
			}
		}
	}
}

func (w *Worker) process(ctx context.Context, component *keb.Component, state cluster.State, schedulingID string) (bool, error) {
	w.logger.Debugf("Processing the reconciliation for a component %s, correlationID: %s",
		component.Component, w.correlationID)
	// check max retry counter
	if w.errorsCount > MaxRetryCount {
		err := w.operationsReg.SetFailed(w.correlationID, schedulingID, "Max retry count reached")
		if err != nil {
			w.logger.Errorf("Error while updating operation status to failed, correlationID %s: %s", w.correlationID, err)
		}
		return true, fmt.Errorf("max retry count for opeation %s in %s excceded", w.correlationID, schedulingID)
	}
	op, _ := w.operationsReg.GetOperation(w.correlationID, schedulingID)
	if op == nil { // New operation
		w.logger.Debugf("Creating new reconciliation operation for a component %s, correlationID: %s",
			component.Component, w.correlationID)
		_, err := w.operationsReg.RegisterOperation(w.correlationID, schedulingID, component.Component, state.Configuration.Version)
		if err != nil {
			return true, fmt.Errorf("error while registering the operation, correlationID %s: %s", w.correlationID, err)
		}

		err = w.callReconciler(ctx, component, state, schedulingID)
		if err != nil {
			w.errorsCount++
			return false, err
		}
		return false, nil
	}

	w.logger.Debugf("Reconciliation operation for a component %s, correlationID: %s has state %s",
		component.Component, w.correlationID, op.State)

	switch op.State {
	case model.OperationStateClientError:
		// In this state we assume that the reconciliation operation was
		// never processed by the component reconciler so we need to call
		// the reconciler again
		err := w.callReconciler(ctx, component, state, schedulingID)
		if err != nil {
			w.errorsCount++
			return false, err
		}
		return false, nil
	case model.OperationStateNew, model.OperationStateInProgress:
		// Operation still being processed by the component reconciler
		return false, nil
	case model.OperationStateError:
		return true, fmt.Errorf("operation errored: %s", op.Reason)
	case model.OperationStateDone:
		err := w.operationsReg.RemoveOperation(w.correlationID, schedulingID)
		if err != nil {
			w.logger.Error("Error while removing the operation, correlationID %s: %s", w.correlationID, err)
		}
		return true, nil
	}
	return false, nil
}

func (w *Worker) callReconciler(ctx context.Context, component *keb.Component, state cluster.State, schedulingID string) error {
	componentsReady, err := w.getDoneComponents(schedulingID)
	if err == nil {
		err = w.invoker.Invoke(ctx, &InvokeParams{
			ComponentToReconcile: component,
			ComponentsReady:      componentsReady,
			ClusterState:         state,
			SchedulingID:         schedulingID,
			CorrelationID:        w.correlationID,
			ReconcilerURL:        w.config.URL,
		})
	}
	if err != nil {
		operr := w.operationsReg.SetClientError(w.correlationID, schedulingID, fmt.Sprintf("Error when calling the reconciler: %s", err))
		if operr != nil {
			w.logger.Errorf("Error while updating operation status to client error, correlationID %s: %s", w.correlationID, err)
		}
		return err
	}

	return nil
}

func (w *Worker) getDoneComponents(schedulingID string) ([]string, error) {
	operations, err := w.operationsReg.GetDoneOperations(schedulingID)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, op := range operations {
		result = append(result, op.Component)
	}
	return result, nil
}

func mapConfiguration(kebCfg []keb.Configuration) map[string]interface{} {
	configs := make(map[string]interface{}, len(kebCfg))
	for _, k := range kebCfg {
		configs[k.Key] = k.Value
	}

	return configs
}
