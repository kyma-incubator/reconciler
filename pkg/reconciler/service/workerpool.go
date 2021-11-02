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

type workPoolBuilder struct {
	workerPool *WorkerPool
	poolSize   int
}

type WorkerPool struct {
	debug        bool
	logger       *zap.SugaredLogger
	antsPool     *ants.Pool
	newRunnerFct func(context.Context, *reconciler.Task, callback.Handler, *zap.SugaredLogger) func() error
	depChecker   *dependencyChecker
}

func newWorkerPoolBuilder(depChecker *dependencyChecker, newRunnerFct func(context.Context, *reconciler.Task, callback.Handler, *zap.SugaredLogger) func() error) *workPoolBuilder {
	return &workPoolBuilder{
		poolSize: defaultWorkers,
		workerPool: &WorkerPool{
			newRunnerFct: newRunnerFct,
			depChecker:   depChecker,
		},
	}
}

func (pb *workPoolBuilder) WithPoolSize(poolSize int) *workPoolBuilder {
	pb.poolSize = poolSize
	return pb
}

func (pb *workPoolBuilder) WithDebug(debug bool) *workPoolBuilder {
	pb.workerPool.debug = debug
	return pb
}

func (pb *workPoolBuilder) Build(ctx context.Context) (*WorkerPool, error) {
	//add logger
	log := logger.NewLogger(pb.workerPool.debug)
	pb.workerPool.logger = log

	//add ants worker pool
	log.Infof("Starting worker pool with %d workers", pb.poolSize)
	antsPool, err := ants.NewPool(pb.poolSize, ants.WithNonblocking(true))
	if err != nil {
		return nil, err
	}
	pb.workerPool.antsPool = antsPool

	go func(ctx context.Context, antsPool *ants.Pool) {
		<-ctx.Done()
		log.Info("Shutting down worker pool")
		antsPool.Release()
	}(ctx, antsPool)

	return pb.workerPool, nil
}

func (wa *WorkerPool) CheckDependencies(model *reconciler.Task) *DependencyCheck {
	return wa.depChecker.newDependencyCheck(model)
}

func (wa *WorkerPool) AssignWorker(ctx context.Context, model *reconciler.Task) error {
	//enrich logger with correlation ID and component name
	loggerNew := logger.NewLogger(wa.debug).With(
		zap.Field{Key: "correlation-id", Type: zapcore.StringType, String: model.CorrelationID},
		zap.Field{Key: "component-name", Type: zapcore.StringType, String: model.Component})

	//create callback handler
	remoteCbh, err := callback.NewRemoteCallbackHandler(model.CallbackURL, loggerNew)
	if err != nil {
		wa.logger.Errorf("Failed to start reconciliation of model '%s'! "+
			"Could not create remote callback handler - not able to process : %s", model, err)
		return err
	}

	//assign runner to worker
	err = wa.antsPool.Submit(func() {
		wa.logger.Debugf("Runner for model '%s' is assigned to worker", model)
		runnerFunc := wa.newRunnerFct(ctx, model, remoteCbh, loggerNew)
		if errRunner := runnerFunc(); errRunner != nil {
			wa.logger.Warnf("Runner failed for model '%s': %v", model, errRunner)
		}
	})

	return err
}
