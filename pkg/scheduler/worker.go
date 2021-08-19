package scheduler

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	DefaultReconciler = "base" //TODO: take this information configurable
	MaxRetryCount     = 20
	MaxDuration       = time.Hour
)

type ReconciliationWorker interface {
	Reconcile(component *keb.Components, state cluster.State, schedulingID string, installCRD bool) error
}

type Worker struct {
	correlationID string
	config        *reconciler.ComponentReconciler
	inventory     cluster.Inventory
	operationsReg OperationsRegistry
	invoker       ReconcilerInvoker
	logger        *zap.SugaredLogger
	errorsCount   int
}

func NewWorker(
	config *reconciler.ComponentReconciler,
	inventory cluster.Inventory,
	operationsReg OperationsRegistry,
	invoker ReconcilerInvoker,
	debug bool) (*Worker, error) {
	log, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &Worker{
		correlationID: uuid.NewString(),
		config:        config,
		inventory:     inventory,
		operationsReg: operationsReg,
		invoker:       invoker,
		logger:        log,
		errorsCount:   0,
	}, nil
}

func (w *Worker) Reconcile(component *keb.Components, state cluster.State, schedulingID string, installCRD bool) error {
	ticker := time.NewTicker(10 * time.Second)
	_, err := w.inventory.UpdateStatus(&state, model.Reconciling)
	if err != nil {
		return errors.Wrap(err, "while updating cluster as reconciling")
	}
	for {
		select {
		case <-time.After(MaxDuration):
			return fmt.Errorf("Max operation time reached for operation %s in %s", w.correlationID, schedulingID)
		case <-ticker.C:
			done, err := w.process(component, state, schedulingID, installCRD)
			if err != nil {
				// At this point something critical happened, we need to give up
				return err
			}
			if done {
				_, err = w.inventory.UpdateStatus(&state, model.Ready)
				if err != nil {
					return errors.Wrap(err, "while updating cluster as ready")
				}
				return nil
			}
		}
	}
}

func (w *Worker) process(component *keb.Components, state cluster.State, schedulingID string, installCRD bool) (bool, error) {
	w.logger.Debugf("Processing the reconciliation for a compoent %s, correlationID: %s", component.Component, w.correlationID)
	// check max retry counter
	if w.errorsCount > MaxRetryCount {
		err := w.operationsReg.SetFailed(w.correlationID, schedulingID, "Max retry count reached")
		if err != nil {
			w.logger.Errorf("Error while updating operation status to failed, correlationID %s: %s", w.correlationID, err)
		}
		return true, fmt.Errorf("Max retry count for opeation %s in %s excceded", w.correlationID, schedulingID)
	}
	op := w.operationsReg.GetOperation(w.correlationID, schedulingID)
	if op == nil { // New operation
		w.logger.Debugf("Creating new reconciliation operation for a component %s, correlationID: %s", component.Component, w.correlationID)
		_, err := w.operationsReg.RegisterOperation(w.correlationID, schedulingID, component.Component)
		if err != nil {
			return true, fmt.Errorf("Error while registering the operation, correlationID %s: %s", w.correlationID, err)
		}

		err = w.callReconciler(component, state, schedulingID, installCRD)
		if err != nil {
			w.errorsCount++
			return false, err
		}
		return false, nil
	}

	w.logger.Debugf("Reconciliation operation for a component %s, correlationID: %s has state %s", component.Component, w.correlationID, op.State)

	switch op.State {
	case StateClientError:
		// In this state we assume that the reconciliation operation was
		// never processed by the component reconciler so we need to call
		// the reconciler again
		err := w.callReconciler(component, state, schedulingID, installCRD)
		if err != nil {
			w.errorsCount++
			return false, err
		}
		return false, nil
	case StateNew, StateInProgress, StateFailed:
		// Operation still being processed by the component reconciler
		return false, nil
	case StateError:
		return true, fmt.Errorf("Operation errored: %s", op.Reason)
	case StateDone:
		err := w.operationsReg.RemoveOperation(w.correlationID, schedulingID)
		if err != nil {
			w.logger.Error("Error while removing the operation, correlationID %s: %s", w.correlationID, err)
		}
		return true, nil
	}
	return false, nil
}

func (w *Worker) callReconciler(component *keb.Components, state cluster.State, schedulingID string, installCRD bool) error {
	var componentsReady []string
	var err error
	if componentsReady, err = w.getDoneComponents(schedulingID); err == nil {
		err = w.invoker.Invoke(&InvokeParams{
			ComponentToReconcile: component,
			ComponentsReady:      componentsReady,
			ClusterState:         state,
			SchedulingID:         schedulingID,
			CorrelationID:        w.correlationID,
			ReconcilerURL:        w.config.URL,
			InstallCRD:           installCRD,
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

func mapConfiguration(kebCfg []keb.Configuration) []reconciler.Configuration {
	reconcilerCfg := make([]reconciler.Configuration, len(kebCfg))
	for _, k := range kebCfg {
		reconcilerCfg = append(reconcilerCfg, reconciler.Configuration{
			Key:   k.Key,
			Value: k.Value,
		})
	}
	return reconcilerCfg
}
