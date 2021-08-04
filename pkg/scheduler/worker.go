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
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"go.uber.org/zap"
)

const DefaultReconciler = "helm"

type ReconciliationWorker interface {
	Reconcile(component *keb.Components, state cluster.State) error
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
}

func NewWorker(config *reconciler.ComponentReconciler, inventory cluster.Inventory, operationsReg OperationsRegistry, debug bool) (*Worker, error) {
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
	}, nil
}

func (w *Worker) Reconcile(component *keb.Components, state cluster.State) error {
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			done, err := w.process(component, state)
			if err != nil {
				// At this point something critical happend, we need to give up
				return err
			}
			if done {
				return nil
			}
		}
	}
}

func (w *Worker) process(component *keb.Components, state cluster.State) (bool, error) {
	// check status
	op := w.operationsReg.GetOperation(w.correlationID)
	if op == nil {
		w.operationsReg.RegisterOperation(w.correlationID)
		// send operation to component reconciler
		err := w.send(component, state)
		if err != nil {
			// TODO
		}
		return false, nil
	}

	switch op.State {
	case StateNew, StateInProgress, StateError:
		return false, nil
	case StateFailed:
		return true, fmt.Errorf("Operation failed: %s", op.Reason)
	case StateDone:
		w.operationsReg.RemoveOperation(w.correlationID)
		return true, nil
	}
	return false, nil
}

func (w *Worker) send(component *keb.Components, state cluster.State) error {
	payload := reconciler.Reconciliation{
		ComponentsReady: []string{},
		Component:       component.Component,
		Namespace:       component.Namespace,
		Version:         state.Configuration.KymaVersion,
		Profile:         state.Configuration.KymaProfile,
		Configuration:   mapConfiguration(component.Configuration),
		Kubeconfig:      state.Cluster.Kubeconfig,
		CallbackURL:     fmt.Sprintf("http://localhost:8080/v1/operations/%s/callback", w.correlationID),
		InstallCRD:      false,
		CorrelationID:   w.correlationID,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Error while marshaling payload for reconciler call: %s", err)
	}

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
	_ = body // TODO

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode != http.StatusPreconditionRequired {
			// something bad happened
		}
	}
	// At this point we can assume that the call was successful
	// and the component reconciler is doing the job of reconciliation
	return nil
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
