package invoker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"strings"
)

const callbackURLTemplate = "%s/v1/operations/%s/callback/%s"

type remoteReconcilerInvoker struct {
	reconRepo     reconciliation.Repository
	mothershipURL string
	reconcilerCfg ComponentReconcilersConfig
	logger        *zap.SugaredLogger
}

func NewRemoteReoncilerInvoker(reconRepo reconciliation.Repository, mothershipURL string, reconcilerCfg ComponentReconcilersConfig, logger *zap.SugaredLogger) *remoteReconcilerInvoker {
	return &remoteReconcilerInvoker{
		reconRepo:     reconRepo,
		mothershipURL: mothershipURL,
		reconcilerCfg: reconcilerCfg,
		logger:        logger,
	}
}

func (i *remoteReconcilerInvoker) Invoke(_ context.Context, params *Params) error {
	resp, err := i.sendHTTPRequest(params)
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			i.logger.Errorf("Error while closing response body: %s", err)
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %s", err)
	}

	i.logger.Debugf("HTTP request to reconciler of component '%s' returned with status '%s' [%d] "+
		"(schedulingID:%s,correlationID:%s) ",
		params.ComponentToReconcile.Component, resp.Status, resp.StatusCode, params.SchedulingID, params.CorrelationID)

	if resp.StatusCode >= http.StatusOK && resp.StatusCode <= 299 {
		var respModel *reconciler.HTTPReconciliationResponse
		if err := i.marshalHTTPResponse(body, respModel, params); err != nil {
			return err
		}
		return i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID,
			model.OperationStateInProgress)
	} else if resp.StatusCode == http.StatusPreconditionRequired {
		var respModel *reconciler.HTTPMissingDependenciesResponse
		if err := i.marshalHTTPResponse(body, respModel, params); err != nil {
			return err
		}
		return i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID,
			model.OperationStateFailed, fmt.Sprintf("dependencies are missing: '%s'",
				strings.Join(respModel.Dependencies.Missing, "', '")))
	} else {
		var respModel *reconciler.HTTPErrorResponse
		if err := i.marshalHTTPResponse(body, respModel, params); err != nil {
			return err
		}
		return i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID,
			model.OperationStateFailed, respModel.Error)
	}
}

func (i *remoteReconcilerInvoker) sendHTTPRequest(params *Params) (*http.Response, error) {
	component := params.ComponentToReconcile.Component

	callbackURL := fmt.Sprintf(callbackURLTemplate, i.mothershipURL, params.SchedulingID, params.CorrelationID)
	payload := params.newRemoteReconciliationModel(callbackURL)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal HTTP payload to call reconciler of component '%s': %s", component, err)
	}

	compRecon, ok := i.reconcilerCfg[component]
	if ok {
		i.logger.Debugf("Found dedicated reconciler for component '%s'", component)
	} else {
		i.logger.Debugf("No dedicated reconciler found for component '%s': "+
			"using '%s' component reconciler as fallback", component, fallbackComponentReconciler)
		compRecon, ok = i.reconcilerCfg[fallbackComponentReconciler]
		if !ok {
			return nil, &NoFallbackReconcilerDefinedError{}
		}
	}

	i.logger.Debugf("Calling remote reconciler via HTTP (URL: %s) for component '%s' (schedulingID:%s,correlationID:%s)",
		compRecon.URL, params.ComponentToReconcile.Component, params.SchedulingID, params.CorrelationID)

	resp, err := http.Post(compRecon.URL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return resp, errors.Wrap(err, fmt.Sprintf("failed to call remote reconciler (URL: %s)", compRecon.URL))
	}

	return resp, nil
}

func (i *remoteReconcilerInvoker) marshalHTTPResponse(body []byte, respModel interface{}, params *Params) error {
	if err := json.Unmarshal(body, respModel); err != nil {
		i.logger.Errorf("Failed to unmarshal HTTP response of reconciler for component '%s': %s",
			params.ComponentToReconcile.Component, err)
		return i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID,
			model.OperationStateClientError, err.Error())
	}
	return nil
}
