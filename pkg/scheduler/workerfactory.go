package scheduler

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"go.uber.org/zap"
)

type WorkerFactory interface {
	ForComponent(component string) (ReconciliationWorker, error)
}

type remoteWorkerFactory struct {
	inventory      cluster.Inventory
	reconcilersCfg reconciler.ComponentReconcilersConfig
	operationsReg  OperationsRegistry
	invoker        *RemoteReconcilerInvoker
	logger         *zap.SugaredLogger
	debug          bool
}

func NewRemoteWorkerFactory(inventory cluster.Inventory, reconcilersCfg reconciler.ComponentReconcilersConfig, operationsReg OperationsRegistry, debug bool) (WorkerFactory, error) {
	l, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &remoteWorkerFactory{
		inventory,
		reconcilersCfg,
		operationsReg,
		&RemoteReconcilerInvoker{logger: l},
		l,
		debug,
	}, nil
}

func (rwf *remoteWorkerFactory) ForComponent(component string) (ReconciliationWorker, error) {
	reconcilerCfg, ok := rwf.reconcilersCfg[component]
	if !ok {
		rwf.logger.Debugf("No reconciler for component %s, using default", component)
		reconcilerCfg = rwf.reconcilersCfg[DefaultReconciler]
	}

	if reconcilerCfg == nil {
		return nil, fmt.Errorf("No reconciler found for component %s", component)
	}
	return NewWorker(reconcilerCfg, rwf.inventory, rwf.operationsReg, rwf.invoker, rwf.debug)
}

type localWorkerFactory struct {
	inventory     cluster.Inventory
	operationsReg OperationsRegistry
	invoker       *LocalReconcilerInvoker
	debug         bool
}

func NewLocalWorkerFactory(inventory cluster.Inventory, operationsReg OperationsRegistry, debug bool) (WorkerFactory, error) {
	l, err := logger.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	return &localWorkerFactory{
		inventory,
		operationsReg,
		&LocalReconcilerInvoker{
			logger:        l,
			operationsReg: operationsReg,
		},
		debug,
	}, nil
}

func (lwf *localWorkerFactory) ForComponent(component string) (ReconciliationWorker, error) {
	return NewWorker(&reconciler.ComponentReconciler{}, lwf.inventory, lwf.operationsReg, lwf.invoker, lwf.debug)
}
