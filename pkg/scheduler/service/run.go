package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/invoker"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/worker"
	"go.uber.org/zap"
	"time"
)

type RuntimeBuilder struct {
	reconRepo        reconciliation.Repository
	logger           *zap.SugaredLogger
	preComponents    []string
	workerPoolConfig *worker.Config
}

func NewRuntimeBuilder(reconRepo reconciliation.Repository, logger *zap.SugaredLogger) *RuntimeBuilder {
	return &RuntimeBuilder{
		reconRepo:        reconRepo,
		logger:           logger,
		workerPoolConfig: &worker.Config{},
	}
}

func (rb *RuntimeBuilder) newWorkerPool(receiver worker.ClusterStateRetriever, invoke invoker.Invoker) (*worker.Pool, error) {
	return worker.NewWorkerPool(receiver, rb.reconRepo, invoke, rb.workerPoolConfig, rb.logger)
}

func (rb *RuntimeBuilder) RunLocal(preComponents []string, statusFunc invoker.ReconcilerStatusFunc) *RunLocal {
	runL := &RunLocal{rb, statusFunc}
	runL.runtimeBuilder.preComponents = preComponents
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

func (rb *RuntimeBuilder) RunRemote(
	conn db.Connection,
	inventory cluster.Inventory,
	config *config.Config) *RunRemote {

	runR := &RunRemote{rb, conn, inventory, config, &SchedulerConfig{}, &BookkeeperConfig{}}
	runR.runtimeBuilder.preComponents = config.Scheduler.PreComponents
	return runR
}

func (rb *RuntimeBuilder) newScheduler() *scheduler {
	return newScheduler(rb.preComponents, rb.logger)
}

type RunLocal struct {
	runtimeBuilder *RuntimeBuilder
	statusFunc     invoker.ReconcilerStatusFunc
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

func (l *RunLocal) Run(ctx context.Context, clusterState *cluster.State) error {
	//enqueue cluster state and create reconciliation entity
	if err := l.runtimeBuilder.newScheduler().RunOnce(clusterState, l.reconciliationRepository()); err == nil {
		l.logger().Debug("Local scheduler finished successfully")
	} else {
		l.logger().Errorf("Local scheduler returned an error: %s", err)
		return err
	}

	//start worker pool
	localInvoker := invoker.NewLocalReconcilerInvoker(l.runtimeBuilder.reconRepo, l.statusFunc, l.logger())
	workerPool, err := l.runtimeBuilder.newWorkerPool(&worker.PassThroughRetriever{State: clusterState}, localInvoker)
	if err != nil {
		l.logger().Errorf("Failed to create worker pool: %s", err)
		return err
	}
	if err := workerPool.RunOnce(ctx); err == nil {
		l.logger().Debug("Worker pool finished successfully")
	} else {
		l.logger().Errorf("Worker pool returned an error: %s", err)
		return err
	}

	return nil
}

type RunRemote struct {
	runtimeBuilder   *RuntimeBuilder
	conn             db.Connection
	inventory        cluster.Inventory
	config           *config.Config
	schedulerConfig  *SchedulerConfig
	bookkeeperConfig *BookkeeperConfig
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

func (r *RunRemote) Run(ctx context.Context) {
	//start bookkeeper
	go func() {
		transition := newClusterStatusTransition(r.conn, r.inventory, r.reconciliationRepository(), r.logger())
		if err := newBookkeeper(transition, r.bookkeeperConfig, r.logger()).Run(ctx); err != nil {
			r.logger().Fatalf("Bookkeeper returned an error: %s", err)
		}
	}()

	//start worker pool
	go func() {
		remoteInvoker := invoker.NewRemoteReoncilerInvoker(r.reconciliationRepository(), r.config, r.logger())
		workerPool, err := r.runtimeBuilder.newWorkerPool(&worker.InventoryRetriever{Inventory: r.inventory}, remoteInvoker)
		if err == nil {
			r.logger().Debug("Worker pool created")
		} else {
			r.logger().Fatalf("Failed to create worker pool: %s", err)
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
}
