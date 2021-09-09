package scheduler

import (
	"context"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
)

type ReconcilerStatusFunc func(component string, msg *reconciler.CallbackMessage)

type LocalReconcilerInvoker struct {
	operationsReg OperationsRegistry
	logger        *zap.SugaredLogger
	statusFunc    ReconcilerStatusFunc
}

func (lri *LocalReconcilerInvoker) Invoke(params *InvokeParams) error {
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

	payload := params.CreateLocalReconciliation(func(msg *reconciler.CallbackMessage) error {
		if lri.statusFunc != nil {
			lri.statusFunc(component, msg)
		}

		switch msg.Status {
		case reconciler.NotStarted, reconciler.Running:
			return lri.operationsReg.SetInProgress(params.CorrelationID, params.SchedulingID)
		case reconciler.Success:
			return lri.operationsReg.SetDone(params.CorrelationID, params.SchedulingID)
		case reconciler.Error:
			return lri.operationsReg.SetError(params.CorrelationID, params.SchedulingID, "Reconciler reported error status")
		}

		return nil
	})

	return componentReconciler.StartLocal(context.Background(), payload)
}
