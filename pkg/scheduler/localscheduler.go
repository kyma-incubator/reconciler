package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"sync/atomic"
)

type LocalSchedulerOption func(*LocalScheduler)

func WithLogger(logger *zap.SugaredLogger) LocalSchedulerOption {
	return func(ls *LocalScheduler) {
		ls.logger = logger
	}
}

func WithPrerequisites(components ...string) LocalSchedulerOption {
	return func(ls *LocalScheduler) {
		ls.prereqs = components
	}
}

func WithStatusFunc(statusFunc ReconcilerStatusFunc) LocalSchedulerOption {
	return func(ls *LocalScheduler) {
		ls.statusFunc = statusFunc
	}
}

type LocalScheduler struct {
	logger        *zap.SugaredLogger
	prereqs       []string
	statusFunc    ReconcilerStatusFunc
	workerFactory WorkerFactory
	installedCRD  int32
}

func NewLocalScheduler(opts ...LocalSchedulerOption) *LocalScheduler {
	ls := &LocalScheduler{
		logger:     zap.NewNop().Sugar(),
		statusFunc: func(component string, msg *reconciler.CallbackMessage) {},
	}

	for _, opt := range opts {
		opt(ls)
	}

	ls.workerFactory = newLocalWorkerFactory(ls.logger, &cluster.MockInventory{}, NewInMemoryOperationsRegistry(), ls.statusFunc)
	return ls
}

func (ls *LocalScheduler) Run(ctx context.Context, c *keb.Cluster) error {
	schedulingID := uuid.NewString()

	clusterState, err := toLocalClusterState(c)
	if err != nil {
		return fmt.Errorf("failed to convert to cluster state: %s", err)
	}

	components, err := clusterState.Configuration.GetComponents()
	if err != nil {
		return fmt.Errorf("failed to get components: %s", err)
	}

	err = ls.reconcilePrereqs(components, clusterState, schedulingID)
	if err != nil {
		return fmt.Errorf("failed to reconcile prerequisite component: %s", err)
	}

	err = ls.reconcileUnprioritizedComponents(ctx, components, clusterState, schedulingID)
	if err != nil {
		return fmt.Errorf("failed to reconcile component: %s", err)
	}

	return nil
}

func toLocalClusterState(c *keb.Cluster) (*cluster.State, error) {
	var defaultContractVersion int64 = 1
	metadata, err := json.Marshal(c.Metadata)
	if err != nil {
		return nil, err
	}

	runtime, err := json.Marshal(c.RuntimeInput)
	if err != nil {
		return nil, err
	}

	clusterEntity := &model.ClusterEntity{
		Cluster:    c.Cluster,
		Runtime:    string(runtime),
		Metadata:   string(metadata),
		Kubeconfig: c.Kubeconfig,
		Contract:   defaultContractVersion,
	}

	components, err := json.Marshal(c.KymaConfig.Components)
	if err != nil {
		return nil, err
	}

	administrators, err := json.Marshal(c.KymaConfig.Administrators)
	if err != nil {
		return nil, err
	}

	configurationEntity := &model.ClusterConfigurationEntity{
		Cluster:        c.Cluster,
		KymaVersion:    c.KymaConfig.Version,
		KymaProfile:    c.KymaConfig.Profile,
		Components:     string(components),
		Administrators: string(administrators),
		Contract:       defaultContractVersion,
	}
	return &cluster.State{
		Cluster:       clusterEntity,
		Configuration: configurationEntity,
		Status:        &model.ClusterStatusEntity{},
	}, nil
}

func (ls *LocalScheduler) reconcilePrereqs(components []*keb.Component, clusterState *cluster.State, schedulingID string) error {
	for _, c := range components {
		if !ls.isPrereq(c) {
			continue
		}

		err := ls.reconcile(c, clusterState, schedulingID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ls *LocalScheduler) reconcileUnprioritizedComponents(ctx context.Context, components []*keb.Component, clusterState *cluster.State, schedulingID string) error {
	g, _ := errgroup.WithContext(ctx)
	for _, c := range components {
		if ls.isPrereq(c) {
			continue
		}

		component := c // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			return ls.reconcile(component, clusterState, schedulingID)
		})
	}

	return g.Wait()
}

func (ls *LocalScheduler) isPrereq(c *keb.Component) bool {
	return contains(ls.prereqs, c.Component)
}

func (ls *LocalScheduler) reconcile(component *keb.Component, state *cluster.State, schedulingID string) error {
	worker, err := ls.workerFactory.ForComponent(component.Component)
	if err != nil {
		return fmt.Errorf("failed to create a worker: %s", err)
	}

	// make sure that installCRD will be only set to true when the first component is scheduled for reconciliation
	installCRD := atomic.CompareAndSwapInt32(&ls.installedCRD, 0, 1)
	err = worker.Reconcile(component, *state, schedulingID, installCRD)
	if err != nil {
		return fmt.Errorf("failed to reconcile a component: %s", component.Component)
	}

	return nil
}

func contains(items []string, item string) bool {
	for i := range items {
		if item == items[i] {
			return true
		}
	}
	return false
}
