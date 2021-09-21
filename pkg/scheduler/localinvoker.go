package scheduler

import (
	"context"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
)

type ReconcilerStatusFunc func(component string, msg *reconciler.CallbackMessage)

type localReconcilerInvoker struct {
	operationsReg OperationsRegistry
	logger        *zap.SugaredLogger
	statusFunc    ReconcilerStatusFunc
}

func (lri *localReconcilerInvoker) Invoke(ctx context.Context, params *InvokeParams) error {
	component := params.ComponentToReconcile.Component

	//resolve component reconciler
	componentReconciler, err := service.GetReconciler(component)
	if err == nil {
		lri.logger.Debugf("Found dedicated component reconciler for component '%s'", component)
	} else {
		lri.logger.Debugf("No dedicated component reconciler found for component '%s': "+
			"using '%s' component reconciler as fallback", component, DefaultReconciler)
		componentReconciler, err = service.GetReconciler(DefaultReconciler)
		if err != nil {
			lri.logger.Errorf("Fallback component reconciler '%s' is missing: "+
				"check local component reconciler initialization", DefaultReconciler)
			return err
		}
	}

	lri.logger.Debugf("Calling the reconciler for a component %s, correlation ID: %s", component, params.CorrelationID)

	model := params.CreateLocalReconciliation(
		lri.createCallbackFunc(params),
	)

	return componentReconciler.StartLocal(ctx, model, lri.logger)
}

func (lri *localReconcilerInvoker) createCallbackFunc(params *InvokeParams) func(msg *reconciler.CallbackMessage) error {
	return func(msg *reconciler.CallbackMessage) error {
		if lri.statusFunc != nil {
			lri.statusFunc(params.ComponentToReconcile.Component, msg)
		}

		switch msg.Status {
		case reconciler.NotStarted, reconciler.Running:
			return lri.operationsReg.SetInProgress(params.CorrelationID, params.SchedulingID)
		case reconciler.Failed:
			return lri.operationsReg.SetFailed(params.CorrelationID, params.SchedulingID,
				fmt.Sprintf("Reconciler reported failure status: %s", msg.Error.Error()))
		case reconciler.Success:
			return lri.operationsReg.SetDone(params.CorrelationID, params.SchedulingID)
		case reconciler.Error:
			return lri.operationsReg.SetError(params.CorrelationID, params.SchedulingID,
				fmt.Sprintf("Reconciler reported error status: %s", msg.Error.Error()))
		}

		return nil
	}
}
