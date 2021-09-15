package scheduler

import (
	"context"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type concurrency bool

const (
	defaultPoolSize                   = 50
	concurrencyNotAllowed concurrency = false
	concurrencyAllowed    concurrency = true
	doNotInstallCRD       bool        = false
	doInstallCRD          bool        = true
)

type Scheduler interface {
	Run(ctx context.Context) error
}

type RemoteScheduler struct {
	inventoryWatch InventoryWatcher
	workerFactory  WorkerFactory
	mothershipCfg  MothershipReconcilerConfig
	poolSize       int
	logger         *zap.SugaredLogger
}

func NewRemoteScheduler(inventoryWatch InventoryWatcher, workerFactory WorkerFactory, mothershipCfg MothershipReconcilerConfig, workers int, debug bool) (Scheduler, error) {
	return &RemoteScheduler{
		inventoryWatch: inventoryWatch,
		workerFactory:  workerFactory,
		mothershipCfg:  mothershipCfg,
		poolSize:       workers,
		logger:         logger.NewLogger(debug),
	}, nil
}

func (rs *RemoteScheduler) validate() error {
	if rs.poolSize < 0 {
		return errors.New("worker pool size cannot be < 0")
	}
	if rs.poolSize == 0 {
		rs.poolSize = defaultPoolSize
	}
	return nil
}

func (rs *RemoteScheduler) Run(ctx context.Context) error {
	if err := rs.validate(); err != nil {
		return err
	}

	queue := make(chan cluster.State, rs.poolSize)

	rs.logger.Debugf("Starting worker pool with capacity %d workers", rs.poolSize)
	workersPool, err := ants.NewPoolWithFunc(rs.poolSize, func(i interface{}) {
		rs.schedule(ctx, i.(cluster.State))
	})
	if err != nil {
		return errors.Wrap(err, "failed to create worker pool of remote-scheduler")
	}

	go func(ctx context.Context, queue chan cluster.State) {
		if err := rs.inventoryWatch.Run(ctx, queue); err != nil {
			rs.logger.Errorf("Failed to run inventory watch: %s", err)
		}
	}(ctx, queue)

	for {
		select {
		case clusterState := <-queue:
			go func(workersPool *ants.PoolWithFunc) {
				if err := workersPool.Invoke(clusterState); err != nil {
					rs.logger.Errorf("Failed to pass cluster to cluster-pool worker: %s", err)
				}
			}(workersPool)
		case <-ctx.Done():
			rs.logger.Debug("Stopping remote scheduler because parent context got closed")
			return nil
		}
	}
}

func (rs *RemoteScheduler) schedule(ctx context.Context, state cluster.State) {
	schedulingID := uuid.NewString()
	components, err := state.Configuration.GetComponents()
	if err != nil {
		rs.logger.Errorf("Failed to get components for cluster %s: %s", state.Cluster.Cluster, err)
		return
	}

	if len(components) == 0 {
		rs.logger.Infof("No components to reconcile for cluster %s", state.Cluster.Cluster)
		return
	}

	statusUpdater := NewClusterStatusUpdater(rs.inventoryWatch.Inventory(), state, components, rs.logger)
	go statusUpdater.Run(ctx)

	//Reconcile CRD components first
	for _, component := range components {
		if rs.isCRDComponent(component.Component) {
			rs.reconcile(component, state, schedulingID, doInstallCRD, concurrencyNotAllowed, statusUpdater)
		}
	}

	//Reconcile pre components
	for _, component := range components {
		if rs.isPreComponent(component.Component) {
			rs.reconcile(component, state, schedulingID, doNotInstallCRD, concurrencyNotAllowed, statusUpdater)
		}
	}

	//Reconcile the rest
	for _, component := range components {
		if rs.isPreComponent(component.Component) || rs.isCRDComponent(component.Component) {
			continue
		}
		rs.reconcile(component, state, schedulingID, doNotInstallCRD, concurrencyAllowed, statusUpdater)
	}
}

func (rs *RemoteScheduler) reconcile(component *keb.Component, state cluster.State, schedulingID string, installCRD bool, concurrent concurrency, statusUpdater ClusterStatusUpdater) {
	fn := func(component *keb.Component, state cluster.State, schedulingID string) {
		worker, err := rs.workerFactory.ForComponent(component.Component)
		if err != nil {
			rs.logger.Errorf("Error creating worker for component: %s", err)
			return
		}
		err = worker.Reconcile(component, state, schedulingID, installCRD)
		if err != nil {
			rs.logger.Errorf("Error while reconciling component %s: %s", component.Component, err)
			statusUpdater.Update(component.Component, model.OperationStateError)
			return
		}
		statusUpdater.Update(component.Component, model.OperationStateDone)
	}

	if bool(concurrent) {
		go fn(component, state, schedulingID)
	} else {
		fn(component, state, schedulingID)
	}
}

func (rs *RemoteScheduler) isCRDComponent(component string) bool {
	for _, c := range rs.mothershipCfg.CrdComponents {
		if component == c {
			return true
		}
	}
	return false
}

func (rs *RemoteScheduler) isPreComponent(component string) bool {
	for _, c := range rs.mothershipCfg.PreComponents {
		if component == c {
			return true
		}
	}
	return false
}
