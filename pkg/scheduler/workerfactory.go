package scheduler

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"go.uber.org/zap"
)

type WorkerFactory interface {
	ForComponent(component string) (ReconciliationWorker, error)
}

type baseWorkerFactory struct {
	inventory     cluster.Inventory
	operationsReg OperationsRegistry
	invoker       ReconcilerInvoker
	logger        *zap.SugaredLogger
	debug         bool
}

type remoteWorkerFactory struct {
	*baseWorkerFactory
	reconcilersCfg reconciler.ComponentReconcilersConfig
	mothershipCfg  reconciler.MothershipReconcilerConfig
}

func NewRemoteWorkerFactory(
	inventory cluster.Inventory,
	reconcilersCfg reconciler.ComponentReconcilersConfig,
	mothershipCfg reconciler.MothershipReconcilerConfig,
	operationsReg OperationsRegistry,
	debug bool) (WorkerFactory, error) {

	log, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}

	return &remoteWorkerFactory{
		&baseWorkerFactory{
			inventory:     inventory,
			operationsReg: operationsReg,
			invoker: &RemoteReconcilerInvoker{
				logger:         log,
				mothershipHost: mothershipCfg.Host,
				mothershipPort: mothershipCfg.Port,
			},
			logger: log,
			debug:  debug,
		},
		reconcilersCfg,
		mothershipCfg,
	}, nil
}

func (rwf *remoteWorkerFactory) ForComponent(component string) (ReconciliationWorker, error) {
	reconcilerCfg, ok := rwf.reconcilersCfg[component]
	if !ok {
		rwf.logger.Debugf("No dedicated component reconciler configured for component '%s': "+
			"using configuration of '%s' component reconciler as fallback", component, DefaultReconciler)
		reconcilerCfg, ok = rwf.reconcilersCfg[DefaultReconciler]
		if !ok {
			rwf.logger.Errorf("Configuration for fallback component reconciler '%s' is missing: "+
				"reconciler confiugration file seems to be incomplete", DefaultReconciler)
		}
	}

	return NewWorker(reconcilerCfg, rwf.inventory, rwf.operationsReg, rwf.invoker, rwf.debug)
}

type localWorkerFactory struct {
	*baseWorkerFactory
}

func NewLocalWorkerFactory(
	inventory cluster.Inventory,
	operationsReg OperationsRegistry,
	statusFunc ReconcilerStatusFunc,
	debug bool) (WorkerFactory, error) {

	log, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}

	return &localWorkerFactory{
		&baseWorkerFactory{
			inventory:     inventory,
			operationsReg: operationsReg,
			invoker: &LocalReconcilerInvoker{
				logger:        log,
				operationsReg: operationsReg,
				statusFunc:    statusFunc,
			},
			logger: log,
			debug:  debug,
		},
	}, nil
}

func (lwf *localWorkerFactory) ForComponent(component string) (ReconciliationWorker, error) {
	return NewWorker(&reconciler.ComponentReconciler{}, lwf.inventory, lwf.operationsReg, lwf.invoker, lwf.debug)
}
