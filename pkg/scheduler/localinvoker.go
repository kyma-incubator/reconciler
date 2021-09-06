package scheduler

import (
	"context"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
)

type ReconcilerStatusFunc func(component string, status reconciler.Status)

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
			}

			return nil
		},
		InstallCRD:    params.InstallCRD,
		CorrelationID: params.CorrelationID,
	})
}
