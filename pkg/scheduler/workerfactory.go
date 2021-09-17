package scheduler

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"go.uber.org/zap"
)

type WorkerFactory interface {
	ForComponent(component string) (ReconciliationWorker, error)
}

type baseWorkerFactory struct {
	inventory     cluster.Inventory
	operationsReg OperationsRegistry
	invoker       reconcilerInvoker
	logger        *zap.SugaredLogger
	debug         bool
}

type remoteWorkerFactory struct {
	*baseWorkerFactory
	reconcilersCfg ComponentReconcilersConfig
	mothershipCfg  MothershipReconcilerConfig
}

func NewRemoteWorkerFactory(
	inventory cluster.Inventory,
	reconcilersCfg ComponentReconcilersConfig,
	mothershipCfg MothershipReconcilerConfig,
	operationsReg OperationsRegistry,
	debug bool) (WorkerFactory, error) {

	log := logger.NewLogger(debug)

	return &remoteWorkerFactory{
		&baseWorkerFactory{
			inventory:     inventory,
			operationsReg: operationsReg,
			invoker: &remoteReconcilerInvoker{
				logger:           log,
				mothershipScheme: mothershipCfg.Scheme,
				mothershipHost:   mothershipCfg.Host,
				mothershipPort:   mothershipCfg.Port,
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
			"using configuration of default component reconciler '%s' as fallback", component, DefaultReconciler)
		reconcilerCfg, ok = rwf.reconcilersCfg[DefaultReconciler]
		if !ok {
			rwf.logger.Errorf("Configuration for default component reconciler '%s' is missing: "+
				"reconciler confiugration file seems to be incomplete", DefaultReconciler)
		}
	}

	return NewWorker(reconcilerCfg, rwf.inventory, rwf.operationsReg, rwf.invoker, rwf.debug)
}

type localWorkerFactory struct {
	*baseWorkerFactory
}

func newLocalWorkerFactory(
	logger *zap.SugaredLogger,
	inventory cluster.Inventory,
	operationsReg OperationsRegistry,
	statusFunc ReconcilerStatusFunc) WorkerFactory {

	return &localWorkerFactory{
		&baseWorkerFactory{
			inventory:     inventory,
			operationsReg: operationsReg,
			invoker: &localReconcilerInvoker{
				logger:        logger,
				operationsReg: operationsReg,
				statusFunc:    statusFunc,
			},
			logger: logger,
		},
	}
}

func (lwf *localWorkerFactory) ForComponent(component string) (ReconciliationWorker, error) {
	//TODO: pass the logger to the worker instead of the debug flag
	return NewWorker(&ComponentReconciler{}, lwf.inventory, lwf.operationsReg, lwf.invoker, true)
}
