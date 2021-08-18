package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
)

type InvokeParams struct {
	ComponentToReconcile *keb.Components
	ComponentsReady      []string
	ClusterState         cluster.State
	SchedulingID         string
	CorrelationID        string
	ReconcilerURL        string
	InstallCRD           bool
}

type ReconcilerInvoker interface {
	Invoke(params *InvokeParams) error
}

type RemoteReconcilerInvoker struct {
	logger         *zap.SugaredLogger
	mothershipHost string
	mothershipPort int
}

func (rri *RemoteReconcilerInvoker) Invoke(params *InvokeParams) error {
	component := params.ComponentToReconcile.Component

	payload := reconciler.Reconciliation{
		ComponentsReady: params.ComponentsReady,
		Component:       component,
		Namespace:       params.ComponentToReconcile.Namespace,
		Version:         params.ClusterState.Configuration.KymaVersion,
		Profile:         params.ClusterState.Configuration.KymaProfile,
		Configuration:   mapConfiguration(params.ComponentToReconcile.Configuration),
		Kubeconfig:      params.ClusterState.Cluster.Kubeconfig,
		CallbackURL:     fmt.Sprintf("http://%s:%d/v1/operations/%s/callback/%s", rri.mothershipHost, rri.mothershipPort, params.SchedulingID, params.CorrelationID), // TODO: parametrize the URL
		InstallCRD:      params.InstallCRD,
		CorrelationID:   params.CorrelationID,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload for reconciler call: %s", err)
	}

	rri.logger.Debugf("Calling the reconciler for a component %s, correlation ID: %s", component, params.CorrelationID)
	resp, err := http.Post(params.ReconcilerURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to call reconciler: %s", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			rri.logger.Errorf("Error while closing the response body: %s", err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read the response body: %s", err)
	}
	rri.logger.Debugf("Called the reconciler for a component %s, correlation ID: %s, got status %s", component, params.CorrelationID, resp.Status)
	_ = body // TODO: handle the reconciler response body

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusPreconditionRequired {
			return fmt.Errorf("failed preconditions: %s", resp.Status)
		}
		return fmt.Errorf("reconciler responded with status: %s", resp.Status)
	}
	// At this point we can assume that the call was successful
	// and the component reconciler is doing the job of reconciliation
	return nil
}

type ReconcilerStatusFunc func(component string, status reconciler.Status)

type LocalReconcilerInvoker struct {
	operationsReg OperationsRegistry
	logger        *zap.SugaredLogger
	statusFunc    ReconcilerStatusFunc
}

func (lri *LocalReconcilerInvoker) Invoke(params *InvokeParams) error {
	component := params.ComponentToReconcile.Component

	componentReconciler, err := service.GetReconciler(component)
	if err != nil {
		return err
	}

	lri.logger.Debugf("Calling the reconciler for a component %s, correlation ID: %s", component, params.CorrelationID)

	return componentReconciler.StartLocal(context.Background(), &reconciler.Reconciliation{
		ComponentsReady: params.ComponentsReady,
		Component:       component,
		Namespace:       params.ComponentToReconcile.Namespace,
		Version:         params.ClusterState.Configuration.KymaVersion,
		Profile:         params.ClusterState.Configuration.KymaProfile,
		Configuration:   mapConfiguration(params.ComponentToReconcile.Configuration),
		Kubeconfig:      params.ClusterState.Cluster.Kubeconfig,
		CallbackFunc: func(status reconciler.Status) error {
			if lri.statusFunc != nil {
				lri.statusFunc(component, status)
			}

			switch status {
			case reconciler.NotStarted, reconciler.Running:
				return lri.operationsReg.SetInProgress(params.CorrelationID, params.SchedulingID)
			case reconciler.Success:
				return lri.operationsReg.SetDone(params.CorrelationID, params.SchedulingID)
			case reconciler.Error:
				return lri.operationsReg.SetError(params.CorrelationID, params.SchedulingID, "Reconciler reported error status")
			case reconciler.Failed:
				return lri.operationsReg.SetFailed(params.CorrelationID, params.SchedulingID, "Reconciler reported failed status")
			}

			return nil
		},
		InstallCRD:    params.InstallCRD,
		CorrelationID: params.CorrelationID,
	})
}
