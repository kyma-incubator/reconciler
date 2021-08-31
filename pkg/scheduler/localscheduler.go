package scheduler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type LocalSchedulerOption func(*LocalScheduler)

func WithLogger(logger *zap.SugaredLogger) LocalSchedulerOption {
	return func(ls *LocalScheduler) {
		ls.logger = logger
	}
}

func WithCRDComponents(components ...string) LocalSchedulerOption {
	return func(ls *LocalScheduler) {
		ls.crdComponents = components
	}
}

func WithPrerequisites(components ...string) LocalSchedulerOption {
	return func(ls *LocalScheduler) {
		ls.prerequisites = components
	}
}

type LocalScheduler struct {
	workerFactory WorkerFactory
	logger        *zap.SugaredLogger
	crdComponents []string
	prerequisites []string
}

func NewLocalScheduler(workerFactory WorkerFactory, opts ...LocalSchedulerOption) *LocalScheduler {
	ls := &LocalScheduler{
		workerFactory: workerFactory,
		logger:        zap.NewNop().Sugar(),
	}

	for _, opt := range opts {
		opt(ls)
	}

	return ls
}

func (ls *LocalScheduler) Run(ctx context.Context, c keb.Cluster) error {
	schedulingID := uuid.NewString()

	clusterState, err := localClusterState(&c)
	if err != nil {
		return fmt.Errorf("failed to convert to cluster state: %s", err)
	}

	components, err := clusterState.Configuration.GetComponents()
	if err != nil {
		return fmt.Errorf("failed to get components: %s", err)
	}

	for _, c := range components {
		if contains(ls.prerequisites, c.Component) {
			ls.reconcile(c, clusterState, schedulingID, false)
		}
	}

	for _, c := range components {
		if contains(ls.crdComponents, c.Component) {
			ls.reconcile(c, clusterState, schedulingID, true)
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, c := range components {
		if contains(ls.crdComponents, c.Component) || contains(ls.prerequisites, c.Component) {
			continue
		}

		component := c // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			return ls.reconcile(component, clusterState, schedulingID, true)
		})
	}

	return g.Wait()
}

func localClusterState(c *keb.Cluster) (*cluster.State, error) {
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

func (ls *LocalScheduler) reconcile(component *keb.Components, state *cluster.State, schedulingID string, installCRD bool) error {
	worker, err := ls.workerFactory.ForComponent(component.Component)
	if err != nil {
		return fmt.Errorf("failed to create a worker: %s", err)
	}

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
