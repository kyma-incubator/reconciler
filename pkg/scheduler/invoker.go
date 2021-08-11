package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"go.uber.org/zap"
)

type InvokeParams struct {
	ComponentToReconcile *keb.Components
	ComponentsReady      []string
	ReconcilerURL        string
	Cluster              cluster.State
	SchedulingID         string
	CorrelationID        string
}

type ReconcilerInvoker interface {
	Invoke(params *InvokeParams) error
}

type RemoteReconcilerInvoker struct {
	logger *zap.SugaredLogger
}

func (rri *RemoteReconcilerInvoker) Invoke(params *InvokeParams) error {
	component := params.ComponentToReconcile.Component

	payload := reconciler.Reconciliation{
		ComponentsReady: params.ComponentsReady,
		Component:       component,
		Namespace:       params.ComponentToReconcile.Namespace,
		Version:         params.Cluster.Configuration.KymaVersion,
		Profile:         params.Cluster.Configuration.KymaProfile,
		Configuration:   mapConfiguration(params.ComponentToReconcile.Configuration),
		Kubeconfig:      params.Cluster.Cluster.Kubeconfig,
		CallbackURL:     fmt.Sprintf("http://localhost:8080/v1/operations/%s/callback/%s", params.SchedulingID, params.CorrelationID), // TODO: parametrize the URL
		InstallCRD:      false,
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
