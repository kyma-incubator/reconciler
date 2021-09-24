package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestInventoryWatch(t *testing.T) {
	clusterStateExpected := mockState()

	//feed mock inventory
	inventory := &cluster.MockInventory{}
	inventory.ClustersToReconcileResult = []*cluster.State{clusterStateExpected}
	queue := make(chan *cluster.State, 1)

	//create inventory watcher
	inventoryWatch, err := newInventoryWatch(inventory, logger.NewLogger(true), &Config{WatchInterval: 500 * time.Millisecond})
	require.NoError(t, err)

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

func TestInventoryWatch_ShouldStopOnCtxClose(t *testing.T) {
	inventory := &cluster.MockInventory{}
	queue := make(chan *cluster.State, 1)
	ctx, cancelFn := context.WithTimeout(context.TODO(), 1500*time.Millisecond)
	defer cancelFn()

	inventoryWatch, err := newInventoryWatch(inventory, logger.NewLogger(true), &Config{WatchInterval: 500 * time.Millisecond})
	require.NoError(t, err)

	startTime := time.Now()
	require.NoError(t, inventoryWatch.Run(ctx, queue))
	require.WithinDuration(t, startTime, time.Now(), 2*time.Second)
}

func mockState() *cluster.State {
	return &cluster.State{
		Cluster: &model.ClusterEntity{
			Version: 1,
			Cluster: "testCluster",
		},
		Configuration: &model.ClusterConfigurationEntity{
			Version:        1,
			Cluster:        "testCluster",
			ClusterVersion: 1,
		},
		Status: &model.ClusterStatusEntity{
			Cluster: "testCluster",
		},
	}
}
