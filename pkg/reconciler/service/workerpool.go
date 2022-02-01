package service

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/occupancy"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
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
	debug         bool
	logger        *zap.SugaredLogger
	antsPool      *ants.Pool
	PoolOccupancy *occupancy.WorkerPoolOccupancy
	callbackURL   string
	newRunnerFct  func(context.Context, *reconciler.Task, callback.Handler, *zap.SugaredLogger, *occupancy.WorkerPoolOccupancy) func() error
}

func newWorkerPoolBuilder(newRunnerFct func(context.Context, *reconciler.Task, callback.Handler, *zap.SugaredLogger, *occupancy.WorkerPoolOccupancy) func() error) *workPoolBuilder {
	return &workPoolBuilder{
		poolSize: defaultWorkers,
		workerPool: &WorkerPool{
			newRunnerFct: newRunnerFct,
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

func (pb *workPoolBuilder) Build(ctx context.Context, occupancyUpdateInterval time.Duration) (*WorkerPool, error) {
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
	pb.workerPool.PoolOccupancy = &occupancy.WorkerPoolOccupancy{
		PoolID: uuid.NewString(),
	}

	go func(ctx context.Context, antsPool *ants.Pool) {
		ticker := time.NewTicker(occupancyUpdateInterval)
		for {
			select {
			case <-ctx.Done():
				log.Info("Shutting down worker pool")
				antsPool.Release()

				occupancyURL, err := occupancyCallbackURL(pb.workerPool.callbackURL, pb.workerPool.PoolOccupancy.PoolID)
				if err == nil {
					client := &http.Client{
						Timeout: 10 * time.Second,
					}
					req, err := http.NewRequest(http.MethodDelete, occupancyURL, nil)
					if err != nil {
						log.Error(err.Error())
					}
					_, err = client.Do(req)
					if err != nil {
						log.Error(err.Error())
					}
				}
				return
			case <-ticker.C:
				log.Debugf("Updating worker pool occupancy from %d to %d", pb.workerPool.PoolOccupancy.RunningWorkers, antsPool.Running())
				pb.workerPool.PoolOccupancy.Lock()
				pb.workerPool.PoolOccupancy.RunningWorkers = antsPool.Running()
				pb.workerPool.PoolOccupancy.Unlock()
			}
		}

	}(ctx, antsPool)

	return pb.workerPool, nil
}

func occupancyCallbackURL(callbackURL string, poolID string) (string, error) {
	if callbackURL == "" {
		return "", fmt.Errorf("error parsing callback URL: received empty string")
	}
	u, err := url.Parse(callbackURL)
	if err != nil {
		return "", err
	}
	occupancyURLTemplate := "%s://%s:%s/v1/occupancy/%s"
	return fmt.Sprintf(occupancyURLTemplate, u.Scheme, u.Hostname(), u.Port(), poolID), nil
}

func (wa *WorkerPool) AssignWorker(ctx context.Context, model *reconciler.Task) error {

	wa.callbackURL = model.CallbackURL
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
		runnerFunc := wa.newRunnerFct(ctx, model, remoteCbh, loggerNew, wa.PoolOccupancy)
		if errRunner := runnerFunc(); errRunner != nil {
			wa.logger.Warnf("Runner failed for model '%s': %v", model, errRunner)
		}
	})

	return err
}

func (wa *WorkerPool) RunningWorkers() int {
	if wa.antsPool == nil {
		return 0
	}
	return wa.antsPool.Running()
}

func (wa *WorkerPool) IsClosed() bool {
	if wa.antsPool == nil {
		return true
	}
	return wa.antsPool.IsClosed()
}
