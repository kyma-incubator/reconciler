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

func (rb *RuntimeBuilder) WithWorkerPoolConfig(cfg *worker.Config) *RuntimeBuilder {
	rb.workerPoolConfig = cfg
	return rb
}

func (rb *RuntimeBuilder) RunLocal(preComponents []string, statusFunc invoker.ReconcilerStatusFunc) *runLocal {
	runL := &runLocal{rb, statusFunc}
	runL.preComponents = preComponents
	return runL
}

func (rb *RuntimeBuilder) RunRemote(
	conn db.Connection,
	inventory cluster.Inventory,
	config *config.Config) *runRemote {

	runR := &runRemote{rb, conn, inventory, config, &SchedulerConfig{}, &BookkeeperConfig{}}
	runR.preComponents = config.Scheduler.PreComponents
	return runR
}

func (rb *RuntimeBuilder) newScheduler() *scheduler {
	return newScheduler(rb.preComponents, rb.logger)
}

type runLocal struct {
	*RuntimeBuilder
	statusFunc invoker.ReconcilerStatusFunc
}

func (l *runLocal) Run(ctx context.Context, clusterState *cluster.State) error {
	//enqueue cluster state and create reconciliation entity
	if err := l.newScheduler().RunOnce(clusterState, l.reconRepo); err == nil {
		l.logger.Debug("Local scheduler finished successfully")
	} else {
		l.logger.Errorf("Local scheduler returned an error: %s", err)
		return err
	}

	//start worker pool
	localInvoker := invoker.NewLocalReconcilerInvoker(l.reconRepo, l.statusFunc, l.logger)
	workerPool, err := l.newWorkerPool(&worker.PassThroughRetriever{State: clusterState}, localInvoker)
	if err != nil {
		l.logger.Errorf("Failed to create worker pool: %s", err)
		return err
	}
	if err := workerPool.RunOnce(ctx); err == nil {
		l.logger.Debug("Worker pool finished successfully")
	} else {
		l.logger.Errorf("Worker pool returned an error: %s", err)
		return err
	}

	return nil
}

type runRemote struct {
	*RuntimeBuilder
	conn             db.Connection
	inventory        cluster.Inventory
	config           *config.Config
	schedulerConfig  *SchedulerConfig
	bookkeeperConfig *BookkeeperConfig
}

func (r *runRemote) WithSchedulerConfig(cfg *SchedulerConfig) *runRemote {
	r.schedulerConfig = cfg
	return r
}

func (r *runRemote) WithBookkeeperConfig(cfg *BookkeeperConfig) *runRemote {
	r.bookkeeperConfig = cfg
	return r
}

func (r *runRemote) Run(ctx context.Context) {
	go func() {
		transition := newClusterStatusTransition(r.conn, r.inventory, r.reconRepo, r.logger)
		if err := newBookkeeper(transition, r.bookkeeperConfig, r.logger).Run(ctx); err != nil {
			r.logger.Fatalf("Bookkeeper returned an error: %s", err)
		}
	}()

	//start worker pool
	go func() {
		remoteInvoker := invoker.NewRemoteReoncilerInvoker(r.reconRepo, r.config, r.logger)
		workerPool, err := r.newWorkerPool(&worker.InventoryRetriever{Inventory: r.inventory}, remoteInvoker)
		if err == nil {
			r.logger.Debug("Worker pool created")
		} else {
			r.logger.Fatalf("Failed to create worker pool: %s", err)
		}

		if err := workerPool.Run(ctx); err != nil {
			r.logger.Fatalf("Worker pool returned an error: %s", err)
		}
	}()

	//start scheduler
	go func() {
		transition := newClusterStatusTransition(r.conn, r.inventory, r.reconRepo, r.logger)
		if err := r.newScheduler().Run(ctx, transition, r.schedulerConfig); err != nil {
			r.logger.Fatalf("Remote scheduler returned an error: %s", err)
		}
	}()
}
