package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
)

func (s *reconciliationTestSuite) TestInventoryWatch() {
	t := s.T()
	clusterStateExpected := &cluster.State{
		Cluster: &model.ClusterEntity{
			Version:   1,
			RuntimeID: "testCluster",
		},
		Configuration: &model.ClusterConfigurationEntity{
			Version:        1,
			RuntimeID:      "testCluster",
			ClusterVersion: 1,
		},
		Status: &model.ClusterStatusEntity{
			RuntimeID: "testCluster",
		},
	}

	//feed mock inventory
	inventory := &cluster.MockInventory{}
	inventory.ClustersToReconcileResult = []*cluster.State{clusterStateExpected}
	queue := make(chan *cluster.State, 1)

	//create inventory watcher
	inventoryWatch := newInventoryWatch(
		inventory,
		logger.NewLogger(true),
		&SchedulerConfig{
			InventoryWatchInterval: 500 * time.Millisecond,
		})

	//stop inventory watcher at the end of the unittest by closing the context
	ctx, cancelFn := context.WithCancel(context.TODO())
	defer cancelFn()

	//start the watcher in the background
	go func(ctx context.Context, queue chan *cluster.State) {
		require.NoError(t, inventoryWatch.Run(ctx, queue))
	}(ctx, queue)

	//wait until watcher found a cluster to reconcile
	clusterStateGot := <-queue

	//verify returned cluster
	require.NotEmpty(t, clusterStateExpected)
	require.Equal(t, clusterStateExpected, clusterStateGot)
}

func (s *reconciliationTestSuite) TestInventoryWatch_ShouldStopOnCtxClose() {
	t := s.T()
	inventory := &cluster.MockInventory{}
	queue := make(chan *cluster.State, 1)
	ctx, cancelFn := context.WithTimeout(context.TODO(), 1500*time.Millisecond)
	defer cancelFn()

	inventoryWatch := newInventoryWatch(
		inventory,
		logger.NewLogger(true),
		&SchedulerConfig{
			InventoryWatchInterval: 500 * time.Millisecond,
		})

	startTime := time.Now()
	require.NoError(t, inventoryWatch.Run(ctx, queue))
	require.WithinDuration(t, startTime, time.Now(), 2*time.Second)
}
