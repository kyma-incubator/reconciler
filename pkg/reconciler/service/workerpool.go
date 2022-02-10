package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	"net/url"
	"time"
)

type workPoolBuilder struct {
	workerPool *WorkerPool
	poolSize   int
}

type WorkerPool struct {
	debug                bool
	logger               *zap.SugaredLogger
	antsPool             *ants.Pool
	PoolID               string
	occupancyCallbackURL string
	newRunnerFct         func(context.Context, *reconciler.Task, callback.Handler, *zap.SugaredLogger) func() error
}

func newWorkerPoolBuilder(newRunnerFct func(context.Context, *reconciler.Task, callback.Handler, *zap.SugaredLogger) func() error) *workPoolBuilder {
	return &workPoolBuilder{
		poolSize: defaultWorkers,
		workerPool: &WorkerPool{
			newRunnerFct: newRunnerFct,
		},
	}
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

func (pb *workPoolBuilder) WithPoolSize(poolSize int) *workPoolBuilder {
	pb.poolSize = poolSize
	return pb
}

func (pb *workPoolBuilder) WithDebug(debug bool) *workPoolBuilder {
	pb.workerPool.debug = debug
	return pb
}

func (pb *workPoolBuilder) Build(ctx context.Context, reconcilerName string) (*WorkerPool, error) {
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
	pb.workerPool.PoolID = uuid.NewString()

	go func(ctx context.Context, antsPool *ants.Pool) {
		ticker := time.NewTicker(defaultInterval)
		for {
			select {
			case <-ctx.Done():
				log.Info("Shutting down worker pool")
				antsPool.Release()
				pb.deleteWorkerPoolOccupancy(log)
				return
			case <-ticker.C:
				if pb.workerPool.occupancyCallbackURL != "" {
					err = pb.updateComponentReconcilerOccupancy(reconcilerName, antsPool.Running())
					if err != nil {
						log.Error(err.Error())
					}
				}
			}
		}

	}(ctx, antsPool)

	return pb.workerPool, nil
}

func (pb *workPoolBuilder) updateComponentReconcilerOccupancy(reconcilerName string, runningWorkers int) error {
	httpOccupancyUpdateRequest := reconciler.HTTPOccupancyRequest{
		Component:      reconcilerName,
		RunningWorkers: runningWorkers,
		PoolSize:       pb.poolSize,
	}
	jsonPayload, err := json.Marshal(httpOccupancyUpdateRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal HTTP payload to update occupancy of component '%s': %s", reconcilerName, err)
	}
	resp, err := http.Post(pb.workerPool.occupancyCallbackURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode <= 299 {
		pb.workerPool.logger.Infof("Component reconciler '%s' updated occupancy successfully", reconcilerName)
	}

	pb.workerPool.logger.Warnf("Mothership failed to update occupancy for '%s' component with status code: '%d'", reconcilerName, resp.StatusCode)

	return nil
}

func (pb *workPoolBuilder) deleteWorkerPoolOccupancy(log *zap.SugaredLogger) {
	if pb.workerPool.occupancyCallbackURL != "" {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		req, err := http.NewRequest(http.MethodDelete, pb.workerPool.occupancyCallbackURL, nil)
		if err != nil {
			log.Error(err.Error())
		}
		_, err = client.Do(req)
		if err != nil {
			log.Error(err.Error())
		}
	}
}

func (wa *WorkerPool) AssignWorker(ctx context.Context, model *reconciler.Task) error {

	//enrich logger with correlation ID and component name
	loggerNew := logger.NewLogger(wa.debug).With(
		zap.Field{Key: "correlation-id", Type: zapcore.StringType, String: model.CorrelationID},
		zap.Field{Key: "component-name", Type: zapcore.StringType, String: model.Component})

	//track occupancyCallbackURL to use it when creating, deleting and updating WP occupancy
	var err error
	wa.occupancyCallbackURL, err = occupancyCallbackURL(model.CallbackURL, wa.PoolID)
	if err != nil {
		wa.logger.Warnf("failed to parse callbackURL '%s': %s", model.CallbackURL, err)
	}

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

func (wa *WorkerPool) IsClosed() bool {
	if wa.antsPool == nil {
		return true
	}
	return wa.antsPool.IsClosed()
}

func (wa *WorkerPool) RunningWorkers() int {
	return wa.antsPool.Running()
}
