package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type WorkerPool struct {
	reconciler *ComponentReconciler
	workerPool *ants.Pool
	ctx        context.Context
}

func newWorkerPool(ctx context.Context, recon *ComponentReconciler) (*WorkerPool, error) {
	//start worker pool
	recon.logger.Infof("Starting worker pool with %d workers", recon.workers)
	workerPool, err := ants.NewPool(recon.workers, ants.WithNonblocking(true))
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		recon.logger.Info("Shutting down worker pool")
		workerPool.Release()
	}()

	return &WorkerPool{
		reconciler: recon,
		workerPool: workerPool,
		ctx:        ctx,
	}, nil
}

func (wa *WorkerPool) AssignWorker(model *reconciler.Reconciliation) error {
	//enrich logger with correlation ID and component name
	loggerNew, err := logger.NewLogger(wa.reconciler.debug)
	if err != nil {
		wa.reconciler.logger.Errorf("Failed to prepare reconciliation of model '%s'! "+
			"Could not create a new logger that is correlationID-aware: %s", model, err)
		return err
	}
	loggerNew = loggerNew.With(
		zap.Field{Key: "correlation-id", Type: zapcore.StringType, String: model.CorrelationID},
		zap.Field{Key: "component-name", Type: zapcore.StringType, String: model.Component})

	//create callback handler
	remoteCbh, err := callback.NewRemoteCallbackHandler(model.CallbackURL, loggerNew)
	if err != nil {
		wa.reconciler.logger.Errorf("Failed to start reconciliation of model '%s'! "+
			"Could not create remote callback handler - not able to process : %s", model, err)
		return err
	}

	//assign runner to worker
	err = wa.workerPool.Submit(func() {
		wa.reconciler.logger.Debugf("Runner for model '%s' is assigned to worker", model)
		runnerFunc := wa.reconciler.newRunnerFunc(wa.ctx, model, remoteCbh)
		if errRunner := runnerFunc(); errRunner != nil {
			wa.reconciler.logger.Warnf("Runner failed for model '%s': %v", model, errRunner)
		}
	})

	return err
}

func (wa *WorkerPool) Reconcilable(model *reconciler.Reconciliation) *ReconcilableResult {
	var missingDeps []string
	for _, compDep := range wa.reconciler.dependencies {
		found := false
		for _, compReady := range model.ComponentsReady {
			if compReady == compDep { //check if required component is part of the components which are ready
				found = true
				break
			}
		}
		if !found {
			missingDeps = append(missingDeps, compDep)
		}
	}
	return &ReconcilableResult{
		Component: model.Component,
		Required:  wa.reconciler.dependencies,
		Missing:   missingDeps,
	}
}

type ReconcilableResult struct {
	Component string
	Required  []string
	Missing   []string
}

func (cd *ReconcilableResult) IsReconcilable() bool {
	return len(cd.Missing) == 0
}
