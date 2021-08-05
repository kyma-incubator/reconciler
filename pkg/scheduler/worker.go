package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
	DefaultReconciler = "helm"
	MaxRetryCount     = 20
	MaxDuration       = time.Hour
)

type ReconciliationWorker interface {
	Reconcile(component *keb.Components, state cluster.State, schedulingID string) error
}

type WorkersFactory struct {
	inventory      cluster.Inventory
	reconcilersCfg reconciler.ComponentReconcilersConfig
	operationsReg  OperationsRegistry
	logger         *zap.SugaredLogger
	debug          bool
}

func NewWorkersFactory(inventory cluster.Inventory, reconcilersCfg reconciler.ComponentReconcilersConfig, operationsReg OperationsRegistry, debug bool) (*WorkersFactory, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &WorkersFactory{
		inventory,
		reconcilersCfg,
		operationsReg,
		logger,
		debug,
	}, nil
}

func (wf *WorkersFactory) ForComponent(component string) (ReconciliationWorker, error) {
	reconcilerCfg, ok := wf.reconcilersCfg[component]
	if !ok {
		wf.logger.Debugf("No reconciler for component %s, using default", component)
		reconcilerCfg = wf.reconcilersCfg[DefaultReconciler]
	}

	if reconcilerCfg == nil {
		return nil, fmt.Errorf("No reconciler found for component %s", component)
	}
	return NewWorker(reconcilerCfg, wf.inventory, wf.operationsReg, wf.debug)
}

type Worker struct {
	correlationID string
	config        *reconciler.ComponentReconciler
	inventory     cluster.Inventory
	operationsReg OperationsRegistry
	logger        *zap.SugaredLogger
	errorsCount   int
}

func NewWorker(
	config *reconciler.ComponentReconciler,
	inventory cluster.Inventory,
	operationsReg OperationsRegistry,
	debug bool) (*Worker, error) {
	logger, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &Worker{
		correlationID: uuid.NewString(),
		config:        config,
		inventory:     inventory,
		operationsReg: operationsReg,
		logger:        logger,
		errorsCount:   0,
	}, nil
}

func (w *Worker) Reconcile(component *keb.Components, state cluster.State, schedulingID string) error {
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
			done, err := w.process(component, state, schedulingID)
			if err != nil {
				// At this point something critical happend, we need to give up
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

func (w *Worker) process(component *keb.Components, state cluster.State, schedulingID string) (bool, error) {
	w.logger.Debugf("Processing the reconciliation for a compoent %s, correlationID: %s", component.Component, w.correlationID)
	// check max retry counter
	if w.errorsCount > MaxRetryCount {
		w.operationsReg.SetFailed(w.correlationID, schedulingID, "Max retry count reached")
		return true, fmt.Errorf("Max retry count for opeation %s in %s excceded", w.correlationID, schedulingID)
	}
	// check status
	op := w.operationsReg.GetOperation(w.correlationID, schedulingID)
	if op == nil { // New operation
		w.logger.Debugf("Creating new reconciliation operation for a component %s, correlationID: %s", component.Component, w.correlationID)
		w.operationsReg.RegisterOperation(w.correlationID, schedulingID, component.Component)
		err := w.callReconciler(component, state, schedulingID)
		if err != nil {
			w.errorsCount += 1
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
		err := w.callReconciler(component, state, schedulingID)
		if err != nil {
			w.errorsCount += 1
			return false, err
		}
		return false, nil
	case StateNew, StateInProgress, StateFailed:
		// Operation still being processed by the component reconciler
		return false, nil
	case StateError:
		return true, fmt.Errorf("Operation errored: %s", op.Reason)
	case StateDone:
		w.operationsReg.RemoveOperation(w.correlationID, schedulingID)
		return true, nil
	}
	return false, nil
}

func (w *Worker) callReconciler(component *keb.Components, state cluster.State, schedulingID string) error {
	err := w.send(component, state, schedulingID)
	if err != nil {
		w.operationsReg.SetClientError(w.correlationID, schedulingID, fmt.Sprintf("Error when calling the reconciler: %s", err))
		return err
	}

	return nil
}

func (w *Worker) send(component *keb.Components, state cluster.State, schedulingID string) error {
	componentsReady, err := w.getDoneComponents(schedulingID)
	if err != nil {
		return err
	}

	payload := reconciler.Reconciliation{
		ComponentsReady: componentsReady,
		Component:       component.Component,
		Namespace:       component.Namespace,
		Version:         state.Configuration.KymaVersion,
		Profile:         state.Configuration.KymaProfile,
		Configuration:   mapConfiguration(component.Configuration),
		Kubeconfig:      state.Cluster.Kubeconfig,
		CallbackURL:     fmt.Sprintf("http://localhost:8080/v1/operations/%s/callback/%s", schedulingID, w.correlationID),
		InstallCRD:      false,
		CorrelationID:   w.correlationID,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Error while marshaling payload for reconciler call: %s", err)
	}

	w.logger.Debugf("Calling the reconciler for a component %s, correlation ID: %s", component.Component, w.correlationID)
	resp, err := http.Post(w.config.URL, "application/json",
		bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("Error while calling reconciler: %s", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			w.logger.Errorf("Error while closing the response body: %s", err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error while reading the response body: %s", err)
	}
	w.logger.Debugf("Called the reconciler for a component %s, correlation ID: %s, got status %s", component.Component, w.correlationID, resp.Status)
	_ = body // TODO: handle the reconciler response body

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusPreconditionRequired {
			return fmt.Errorf("Preconditions failed: %s", resp.Status)
		}
		return fmt.Errorf("Reconciler responded with status: %s", resp.Status)
	}
	// At this point we can assume that the call was successful
	// and the component reconciler is doing the job of reconciliation
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
