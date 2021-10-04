package invoker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"
)

const callbackURLTemplate = "%s://%s:%d/v1/operations/%s/callback/%s"

type RemoteReconcilerInvoker struct {
	reconRepo reconciliation.Repository
	config    *config.Config
	logger    *zap.SugaredLogger
}

func NewRemoteReoncilerInvoker(reconRepo reconciliation.Repository, cfg *config.Config, logger *zap.SugaredLogger) *RemoteReconcilerInvoker {
	return &RemoteReconcilerInvoker{
		reconRepo: reconRepo,
		config:    cfg,
		logger:    logger,
	}
}

func (i *RemoteReconcilerInvoker) Invoke(_ context.Context, params *Params) error {
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

	if resp.StatusCode >= http.StatusOK && resp.StatusCode <= 299 {
		//component-reconciler started reconciliation
		respModel := &reconciler.HTTPReconciliationResponse{}
		err := i.marshalHTTPResponse(body, respModel, params)
		if err == nil {
			return i.updateOperationState(params, model.OperationStateInProgress)
		}
		i.reportUnmarshalError(resp.StatusCode, body, err)
	}

	if resp.StatusCode == http.StatusPreconditionRequired {
		//component-reconciler can not start because dependencies are missing
		respModel := &reconciler.HTTPMissingDependenciesResponse{}
		err := i.marshalHTTPResponse(body, respModel, params)
		if err == nil {
			return i.updateOperationState(params, model.OperationStateFailed,
				fmt.Sprintf("dependencies are missing: '%s'", strings.Join(respModel.Dependencies.Missing, "', '")))
		}
		i.reportUnmarshalError(resp.StatusCode, body, err)
	}

	if resp.StatusCode >= 400 && resp.StatusCode <= 499 {
		//component-reconciler can not start because dependencies are missing
		respModel := &reconciler.HTTPErrorResponse{}
		err := i.marshalHTTPResponse(body, respModel, params)
		if err == nil {
			return i.updateOperationState(params, model.OperationStateFailed, respModel.Error)
		}
		i.reportUnmarshalError(resp.StatusCode, body, err)
	}

	//component-reconciler responded an error: try to handle it as an error response
	respModel := &reconciler.HTTPErrorResponse{}
	var errorReason string

	err = i.marshalHTTPResponse(body, respModel, params)
	if err == nil {
		errorReason = respModel.Error
	} else {
		i.reportUnmarshalError(resp.StatusCode, body, err)
		errorReason = fmt.Sprintf("received unsupported reconciler response (HTTP code: %d): %s",
			resp.StatusCode, string(body))
	}

	return i.updateOperationState(params, model.OperationStateClientError, errorReason)
}

func (i *RemoteReconcilerInvoker) reportUnmarshalError(httpCode int, body []byte, err error) {
	i.logger.Warnf("Remote invoker: Failed to unmarshal reconciler response (HTTP-code: %d / Body: %s): %s",
		httpCode, string(body), err)
}

func (i *RemoteReconcilerInvoker) sendHTTPRequest(params *Params) (*http.Response, error) {
	component := params.ComponentToReconcile.Component

	callbackURL := fmt.Sprintf(callbackURLTemplate,
		i.config.Scheme,
		i.config.Host,
		i.config.Port,
		params.SchedulingID,
		params.CorrelationID)
	payload := params.newRemoteReconciliationModel(callbackURL)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal HTTP payload to call reconciler of component '%s': %s", component, err)
	}

	compRecon, ok := i.config.Scheduler.Reconcilers[component]
	if ok {
		i.logger.Debugf("Remote invoker found dedicated reconciler for component '%s'", component)
	} else {
		i.logger.Debugf("Remote invoker found no dedicated reconciler for component '%s': "+
			"using '%s' component reconciler as fallback", component, config.FallbackComponentReconciler)
		compRecon, ok = i.config.Scheduler.Reconcilers[config.FallbackComponentReconciler]
		if !ok {
			i.logger.Errorf("Remote invoker could not find fallback reconciler '%s' in scheduler configuration",
				config.FallbackComponentReconciler)
			return nil, &NoFallbackReconcilerDefinedError{}
		}
	}

	i.logger.Debugf("Remote invoker is calling remote reconciler via HTTP (URL: %s) "+
		"for component '%s' (schedulingID:%s/correlationID:%s)",
		compRecon.URL, params.ComponentToReconcile.Component, params.SchedulingID, params.CorrelationID)

	resp, err := http.Post(compRecon.URL, "application/json", bytes.NewBuffer(jsonPayload))
	if err == nil {
		respDump, err := httputil.DumpResponse(resp, true)
		if err == nil {
			i.logger.Debugf("Remote invoker received HTTP response from reconciler of component '%s' with status '%s' [%d] "+
				"(schedulingID:%s/correlationID:%s): %s",
				params.ComponentToReconcile.Component, resp.Status, resp.StatusCode,
				params.SchedulingID, params.CorrelationID, string(respDump))
		} else {
			i.logger.Warnf("Remote invoker failed to dump HTTP response from component reconciler: %s", err)
		}
	} else {
		return resp, errors.Wrap(err, fmt.Sprintf("failed to call remote reconciler (URL: %s)", compRecon.URL))
	}

	return resp, nil
}

func (i *RemoteReconcilerInvoker) marshalHTTPResponse(body []byte, respModel interface{}, params *Params) error {
	if err := json.Unmarshal(body, respModel); err != nil {
		i.logger.Errorf("Remote invoker failed to unmarshal HTTP response of reconciler for component '%s': %s",
			params.ComponentToReconcile.Component, err)

		//update the operation to be failed caused by client error
		errUpdState := i.updateOperationState(params, model.OperationStateClientError, err.Error())
		if errUpdState != nil {
			err = errors.Wrap(err, fmt.Sprintf("failed to update state of operation (scheudlingID:%s/correlationID:%s) to '%s': %s",
				params.SchedulingID, params.CorrelationID, model.OperationStateClientError, errUpdState))
		}

		return err
	}
	return nil
}

func (i *RemoteReconcilerInvoker) updateOperationState(params *Params, state model.OperationState, reasons ...string) error {
	err := i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID, state, strings.Join(reasons, ", "))
	if err != nil {
		if reconciliation.IsRedundantOperationStateUpdateError(err) {
			i.logger.Debugf("Remote invoker tried an redundant update of operation (scheudlingID:%s/correlationID:%s) "+
				"to state '%s'", params.SchedulingID, params.CorrelationID, state)
		} else {
			return errors.Wrap(err, fmt.Sprintf("remote invoker failed to update operation "+
				"(scheudlingID:%s/correlationID:%s) to state '%s'", params.SchedulingID, params.CorrelationID, state))
		}
	}
	return nil
}
