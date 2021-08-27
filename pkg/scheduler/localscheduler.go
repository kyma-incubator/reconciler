package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"go.uber.org/zap"
)

type LocalSchedulerOption func(*LocalScheduler)

func WithLogger(l *zap.SugaredLogger) LocalSchedulerOption {
	return func(ls *LocalScheduler) {
		ls.logger = l
	}
}

type LocalScheduler struct {
	cluster       keb.Cluster
	workerFactory WorkerFactory
	logger        *zap.SugaredLogger
}

func NewLocalScheduler(cluster keb.Cluster, workerFactory WorkerFactory, opts ...LocalSchedulerOption) (Scheduler, error) {
	l, err := logger.NewLogger(false)
	if err != nil {
		return nil, err
	}

	ls := &LocalScheduler{
		cluster:       cluster,
		workerFactory: workerFactory,
		logger:        l,
	}

	for _, opt := range opts {
		opt(ls)
	}

	return ls, nil
}

func (ls *LocalScheduler) Run(ctx context.Context) error {
	schedulingID := uuid.NewString()

	clusterState, err := localClusterState(&ls.cluster)
	if err != nil {
		return fmt.Errorf("failed to convert to cluster state: %s", err)
	}

	components, err := clusterState.Configuration.GetComponents()
	if err != nil {
		return fmt.Errorf("failed to get components: %s", err)
	}

	results := make(chan error, len(components))

	var wg sync.WaitGroup
	wg.Add(len(components))

	//trigger all component reconcilers
	for _, component := range components {
		worker, err := ls.workerFactory.ForComponent(component.Component)
		if err != nil {
			return fmt.Errorf("failed to create a: %s", err)
		}

		go func(component *keb.Components, state cluster.State, schedulingID string) {
			defer wg.Done()
			err := worker.Reconcile(component, state, schedulingID, true)
			if err != nil {
				ls.logger.Errorf("Error while reconciling component %s: %s", component.Component, err)
			}
			results <- err
		}(component, *clusterState, schedulingID)
	}

	wg.Wait()

	close(results)

	for err := range results {
		if err != nil {
			return err
		}
	}

	return nil
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
