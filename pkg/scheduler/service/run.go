package service

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/features"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"time"

	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/invoker"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/worker"
)

type RuntimeBuilder struct {
	reconRepo        reconciliation.Repository
	logger           *zap.SugaredLogger
	workerPoolConfig *worker.Config
}

func NewRuntimeBuilder(reconRepo reconciliation.Repository, logger *zap.SugaredLogger) *RuntimeBuilder {
	return &RuntimeBuilder{
		reconRepo:        reconRepo,
		logger:           logger,
		workerPoolConfig: &worker.Config{},
	}
}

func (rb *RuntimeBuilder) newWorkerPool(retriever worker.ClusterStateRetriever, invoke invoker.Invoker) (*worker.Pool, error) {
	return worker.NewWorkerPool(retriever, rb.reconRepo, invoke, rb.workerPoolConfig, rb.logger)
}

func (rb *RuntimeBuilder) RunLocal(statusFunc invoker.ReconcilerStatusFunc) *RunLocal {
	runL := &RunLocal{runtimeBuilder: rb, statusFunc: statusFunc}

	//Make sure local runner will NOT retry if the local invoker returns an error!
	//If retries are enabled, operations which are reaching a final state (e.g. 'error') would try to switch back
	//to 'running' or another interim state which is not allowed and causes errors.
	runL.runtimeBuilder.workerPoolConfig = &worker.Config{
		PoolSize:               30,              //should be sufficient for a local installation
		OperationCheckInterval: 1 * time.Second, //only used by bookkeeper which isn't running for local installation
		InvokerMaxRetries:      1,               //don't retry!
		InvokerRetryDelay:      1 * time.Second,
	}
	return runL
}

func (rb *RuntimeBuilder) RunRemote(conn db.Connection, inventory cluster.Inventory, occupancyRepo occupancy.Repository, config *config.Config) *RunRemote {

	runR := &RunRemote{
		runtimeBuilder:   rb,
		conn:             conn,
		inventory:        inventory,
		occupancyRepo:    occupancyRepo,
		config:           config,
		schedulerConfig:  &SchedulerConfig{},
		bookkeeperConfig: &BookkeeperConfig{},
		cleanerConfig:    &CleanerConfig{},
	}
	return runR
}

func (rb *RuntimeBuilder) newScheduler() *scheduler {
	return newScheduler(rb.logger)
}

func (rb *RuntimeBuilder) newCleaner() *cleaner {
	return newCleaner(rb.logger)
}

type RunLocal struct {
	runtimeBuilder  *RuntimeBuilder
	statusFunc      invoker.ReconcilerStatusFunc
	schedulerConfig *SchedulerConfig
}

func (l *RunLocal) logger() *zap.SugaredLogger { //convenient function
	return l.runtimeBuilder.logger
}

func (l *RunLocal) reconciliationRepository() reconciliation.Repository { //convenient function
	return l.runtimeBuilder.reconRepo
}

func (l *RunLocal) WithWorkerPoolSize(size int) *RunLocal {
	l.runtimeBuilder.workerPoolConfig.PoolSize = size
	return l
}

func (l *RunLocal) WithWorkerPoolMaxRetries(maxOperationsRetries int) *RunLocal {
	l.runtimeBuilder.workerPoolConfig.MaxOperationRetries = maxOperationsRetries
	return l
}

func (l *RunLocal) WithSchedulerConfig(cfg *SchedulerConfig) *RunLocal {
	l.schedulerConfig = cfg
	return l
}

func (l *RunLocal) Run(ctx context.Context, clusterState *cluster.State) (*ReconciliationResult, error) {
	if err := l.schedulerConfig.validate(); err != nil {
		return nil, err
	}
	//enqueue cluster state and create reconciliation entity
	l.logger().Info("Starting local scheduler")
	if err := l.runtimeBuilder.newScheduler().RunOnce(clusterState, l.reconciliationRepository(), l.schedulerConfig); err == nil {
		l.logger().Info("Local scheduler finished successfully")
	} else {
		l.logger().Errorf("Local scheduler returned an error: %s", err)
		return nil, err
	}

	//start worker pool
	l.logger().Info("Starting worker pool")
	localInvoker := invoker.NewLocalReconcilerInvoker(l.runtimeBuilder.reconRepo, l.statusFunc, l.logger())
	workerPool, err := l.runtimeBuilder.newWorkerPool(&worker.PassThroughRetriever{State: clusterState}, localInvoker)
	if err != nil {
		l.logger().Errorf("Failed to create worker pool: %s", err)
		return nil, err
	}
	if err := workerPool.RunOnce(ctx); err == nil {
		l.logger().Info("Worker pool finished successfully")
	} else {
		l.logger().Errorf("Worker pool returned an error: %s", err)
		return nil, err
	}

	//retrieve reconciliation model
	recons, err := l.reconciliationRepository().GetReconciliations(&reconciliation.WithRuntimeID{
		RuntimeID: clusterState.Cluster.RuntimeID,
	})
	if err != nil {
		return nil, err
	}
	if len(recons) != 1 {
		return nil, fmt.Errorf("illegal state detected: found %d reconciliations for cluster with runtimeID '%s'",
			len(recons), clusterState.Cluster.RuntimeID)
	}

	//retrieve operation models
	ops, err := l.reconciliationRepository().GetOperations(&operation.WithSchedulingID{
		SchedulingID: recons[0].SchedulingID,
	})
	if err != nil {
		return nil, err
	}

	//evaluate reconciliation result
	reconResult := newReconciliationResult(recons[0], l.logger())
	err = reconResult.AddOperations(ops)
	return reconResult, err
}

type RunRemote struct {
	runtimeBuilder   *RuntimeBuilder
	conn             db.Connection
	inventory        cluster.Inventory
	occupancyRepo    occupancy.Repository
	config           *config.Config
	schedulerConfig  *SchedulerConfig
	bookkeeperConfig *BookkeeperConfig
	cleanerConfig    *CleanerConfig
}

func (r *RunRemote) logger() *zap.SugaredLogger { //convenient function
	return r.runtimeBuilder.logger
}

func (r *RunRemote) reconciliationRepository() reconciliation.Repository { //convenient function
	return r.runtimeBuilder.reconRepo
}

func (r *RunRemote) WithWorkerPoolConfig(cfg *worker.Config) *RunRemote {
	r.runtimeBuilder.workerPoolConfig = cfg
	return r
}

func (r *RunRemote) WithSchedulerConfig(cfg *SchedulerConfig) *RunRemote {
	r.schedulerConfig = cfg
	return r
}

func (r *RunRemote) WithBookkeeperConfig(cfg *BookkeeperConfig) *RunRemote {
	r.bookkeeperConfig = cfg
	return r
}

func (r *RunRemote) WithCleanerConfig(cfg *CleanerConfig) *RunRemote {
	r.cleanerConfig = cfg
	return r
}

func (r *RunRemote) Run(ctx context.Context) error {
	if err := r.config.Validate(); err != nil {
		return err
	}
	//start bookkeeper
	go func() {
		transition := newClusterStatusTransition(r.conn, r.inventory, r.reconciliationRepository(), r.logger())
		if err := newBookkeeper(transition.reconRepo, r.bookkeeperConfig, r.logger()).Run(ctx,
			markOrphanOperation{transition: transition, logger: r.logger()},
			finishOperation{transition: transition, logger: r.logger()}); err != nil {
			r.logger().Fatalf("Bookkeeper returned an error: %s", err)
		}
	}()

	//start worker pool
	go func() {
		remoteInvoker := invoker.NewRemoteReconcilerInvoker(r.reconciliationRepository(), r.config, r.logger())
		workerPool, err := r.runtimeBuilder.newWorkerPool(&worker.InventoryRetriever{Inventory: r.inventory}, remoteInvoker)
		if err == nil {
			r.logger().Info("Worker pool created")
		} else {
			r.logger().Fatalf("Failed to create worker pool: %s", err)
		}
		if features.Enabled(features.WorkerpoolOccupancyTracking) {
			//start occupancy tracker to track worker pool
			err = NewOccupancyTracker(workerPool, r.occupancyRepo, r.config.Scheduler.Reconcilers, r.logger()).Run(ctx)
			if err == nil {
				r.logger().Info("Occupancy tracker started")
			} else {
				r.logger().Errorf("Occupancy tracker failed to start: %s", err)
			}
		}

		if err := workerPool.Run(ctx); err != nil {
			r.logger().Fatalf("Worker pool returned an error: %s", err)
		}
	}()

	//start scheduler
	go func() {
		transition := newClusterStatusTransition(r.conn, r.inventory, r.reconciliationRepository(), r.logger())
		if err := r.runtimeBuilder.newScheduler().Run(ctx, transition, r.schedulerConfig); err != nil {
			r.logger().Fatalf("Remote scheduler returned an error: %s", err)
		}
	}()

	//start cleaner
	go func() {
		transition := newClusterStatusTransition(r.conn, r.inventory, r.reconciliationRepository(), r.logger())
		if err := r.runtimeBuilder.newCleaner().Run(ctx, transition, r.cleanerConfig); err != nil {
			r.logger().Fatalf("Cleaner returned an error: %s", err)
		}
	}()

	return nil
}
