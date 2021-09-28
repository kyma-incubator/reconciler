package invoker

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	reconRegistry "github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
)

//ReconcilerStatusFunc can be passed by caller to retrieve status updates
type ReconcilerStatusFunc func(component string, msg *reconciler.CallbackMessage)

type localReconcilerInvoker struct {
	reconRepo  reconciliation.Repository
	logger     *zap.SugaredLogger
	statusFunc ReconcilerStatusFunc
}

func NewLocalReconcilerInvoker(reconRepo reconciliation.Repository, statusFunc ReconcilerStatusFunc, logger *zap.SugaredLogger) *localReconcilerInvoker {
	return &localReconcilerInvoker{
		reconRepo:  reconRepo,
		logger:     logger,
		statusFunc: statusFunc,
	}
}

func (i *localReconcilerInvoker) Invoke(ctx context.Context, params *Params) error {
	component := params.ComponentToReconcile.Component

	//resolve component reconciler
	compRecon, err := reconRegistry.GetReconciler(component)
	if err == nil {
		i.logger.Debugf("Found dedicated reconciler for component '%s'", component)
	} else {
		i.logger.Debugf("No dedicated reconciler found for component '%s': "+
			"using '%s' reconciler as fallback", component, config.FallbackComponentReconciler)
		compRecon, err = reconRegistry.GetReconciler(config.FallbackComponentReconciler)
		if err != nil {
			return &NoFallbackReconcilerDefinedError{}
		}
	}

	i.logger.Debugf("Calling reconciler for component '%s' locally (schedulingID:%s/correlationID:%s)",
		component, params.SchedulingID, params.CorrelationID)

	reconModel := params.newLocalReconciliationModel(i.newCallbackFunc(params))

	return compRecon.StartLocal(ctx, reconModel, i.logger)
}

func (i *localReconcilerInvoker) newCallbackFunc(params *Params) func(msg *reconciler.CallbackMessage) error {
	return func(msg *reconciler.CallbackMessage) error {
		if i.statusFunc == nil {
			i.logger.Debugf("Status-func undefined: no status updates for component '%s' will be send to caller",
				params.ComponentToReconcile.Component)
		} else {
			i.logger.Debugf("Caller provided status-func: sending status-update '%s' (error: '%s') for component '%s' to caller",
				msg.Status, *msg.Error, params.ComponentToReconcile.Component)
			i.statusFunc(params.ComponentToReconcile.Component, msg)
		}

		switch msg.Status {
		case reconciler.StatusNotstarted, reconciler.StatusRunning:
			return i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID,
				model.OperationStateInProgress)
		case reconciler.StatusFailed:
			return i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID,
				model.OperationStateFailed, *msg.Error)
		case reconciler.StatusSuccess:
			return i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID,
				model.OperationStateDone)
		case reconciler.StatusError:
			return i.reconRepo.UpdateOperationState(params.SchedulingID, params.CorrelationID,
				model.OperationStateError, *msg.Error)
		}

		return nil
	}
}
