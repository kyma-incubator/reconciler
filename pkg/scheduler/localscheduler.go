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

	components, err := clusterState.Configuration.GetComponents(ls.prereqs)
	if err != nil {
		return fmt.Errorf("failed to get components: %s", err)
	}

	if components == nil {
		ls.logger.Infof("No components to reconcile for cluster %s", c.Cluster)
		return nil
	}

	handler := NewReconciliationHandler(ls.workerFactory)
	err = handler.Reconcile(ctx, components, clusterState, schedulingID)
	if err != nil {
		return fmt.Errorf("failed to reconcile components for cluster %s: %s", c.Cluster, err)
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
