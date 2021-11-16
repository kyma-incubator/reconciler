package invoker

import (
	"context"
	"fmt"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/pkg/errors"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	reconRegistry "github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
)

//ReconcilerStatusFunc can be passed by caller to retrieve status updates
type ReconcilerStatusFunc func(component string, msg *reconciler.CallbackMessage)

type LocalReconcilerInvoker struct {
	reconRepo  reconciliation.Repository
	logger     *zap.SugaredLogger
	statusFunc ReconcilerStatusFunc
}

func NewLocalReconcilerInvoker(reconRepo reconciliation.Repository, statusFunc ReconcilerStatusFunc, logger *zap.SugaredLogger) *LocalReconcilerInvoker {
	return &LocalReconcilerInvoker{
		reconRepo:  reconRepo,
		logger:     logger,
		statusFunc: statusFunc,
	}
}

func (i *LocalReconcilerInvoker) Invoke(ctx context.Context, params *Params) error {
	if params.ComponentToReconcile == nil {
		return fmt.Errorf("illegal state: local invoker was called without providing a component to reconcile "+
			"(schedulingID:%s/correlationID:%s)", params.SchedulingID, params.CorrelationID)
	}
	component := params.ComponentToReconcile.Component

	//resolve component reconciler
	compRecon, err := reconRegistry.GetReconciler(component)
	if err == nil {
		i.logger.Debugf("Local invoker found dedicated reconciler for component '%s'", component)
	} else {
		i.logger.Debugf("Local invoker could not find a dedicated reconciler for component '%s': "+
			"using '%s' reconciler as fallback", component, config.FallbackComponentReconciler)
		compRecon, err = reconRegistry.GetReconciler(config.FallbackComponentReconciler)
		if err != nil {
			registeredRecons := reconRegistry.RegisteredReconcilers()
			i.logger.Errorf("Local invoker could not find fallback component reconciler '%s' in reconciler registry "+
				"(available are: '%s')", config.FallbackComponentReconciler, strings.Join(registeredRecons, "', '"))
			return &NoFallbackReconcilerDefinedError{}
		}
	}

	i.logger.Debugf("Local invoker is calling reconciler for component '%s' (schedulingID:%s/correlationID:%s)",
		component, params.SchedulingID, params.CorrelationID)

	reconModel := params.newLocalTask(i.newCallbackFunc(params))

	return compRecon.StartLocal(ctx, reconModel, i.logger)
}

func (i *LocalReconcilerInvoker) newCallbackFunc(params *Params) func(msg *reconciler.CallbackMessage) error {
	return func(msg *reconciler.CallbackMessage) error {
		if i.statusFunc == nil {
			i.logger.Debugf("Local invoker has no Status-func configured: "+
				"no status updates for component '%s' will be send to caller",
				params.ComponentToReconcile.Component)
		} else {
			i.logger.Debugf("Local invoker has Status-func configured: sending status-update '%s' (error: '%s') "+
				"for component '%s' to caller",
				msg.Status, msg.Error, params.ComponentToReconcile.Component)
			i.statusFunc(params.ComponentToReconcile.Component, msg)
		}

		//Mark the operation to be running or in failure state.
		//Be aware that final states (Done, Error) for an operation will be set by worker
		//because the worker controls retries etc. The invoker should only set interim states
		//for an operation (e.g. failed, running client-error etc.).
		switch msg.Status {
		case reconciler.StatusRunning:
			return i.updateOperationState(msg, params, model.OperationStateInProgress)
		case reconciler.StatusFailed:
			return i.updateOperationState(msg, params, model.OperationStateFailed)
		case reconciler.StatusError:
			return i.updateOperationState(msg, params, model.OperationStateError)
		case reconciler.StatusSuccess:
			return i.updateOperationState(msg, params, model.OperationStateDone)
		default:
			i.logger.Debugf("Local invoker reported operation status '%s' but will not propagate "+
				"it as new state to operation (schedulingID:%s/correlationID:%s)",
				msg.Status, params.SchedulingID, params.CorrelationID)
		}

		return nil
	}
}

func (i *LocalReconcilerInvoker) updateOperationState(msg *reconciler.CallbackMessage, params *Params, state model.OperationState) error {
	errMsg := "Local invoker is updating operation (schedulingID:%s/correlationID:%s) to state '%s'"
	if msg.Error == "" {
		i.logger.Debugf(errMsg, params.SchedulingID, params.CorrelationID, state)
	} else {
		i.logger.Debugf(errMsg+": %s", params.SchedulingID, params.CorrelationID, state, msg.Error)
	}

	err := i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID, state, msg.Error)
	if err != nil {
		//return only the error if it's not caused by a redundant update
		return errors.Wrap(err, fmt.Sprintf("local invoker failed to update operation "+
			"(schedulingID:%s/correlationID:%s) to state '%s'",
			params.SchedulingID, params.CorrelationID, state))
	}
	return nil
}
