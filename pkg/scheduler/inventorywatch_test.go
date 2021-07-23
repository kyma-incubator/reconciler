package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInventoryWatch(t *testing.T) {
	inventory := &cluster.MockInventory{}
	dummyState := mockState()
	inventory.ClustersToReconcileResult = []*cluster.State{dummyState}
	queue := make(chan cluster.State, 1)
	ctx, cancelFn := context.WithCancel(context.TODO())
	defer cancelFn()

	inventoryWatch, err := NewInventoryWatch(inventory, true, &InventoryWatchConfig{WatchInterval: 500 * time.Millisecond})
	require.NoError(t, err)

	go func(ctx context.Context, queue chan cluster.State) {
		err := inventoryWatch.Run(ctx, queue)
		require.NoError(t, err)
	}(ctx, queue)
	clusterState := <-queue

	require.NotEmpty(t, clusterState)
	require.Equal(t, "foo", clusterState.Status.Cluster)
}

func TestInventoryWatch_ShouldStopOnCtxClose(t *testing.T) {
	inventory := &cluster.MockInventory{}
	queue := make(chan cluster.State, 1)
	ctx, cancelFn := context.WithCancel(context.TODO())

	inventoryWatch, err := NewInventoryWatch(inventory, true, &InventoryWatchConfig{WatchInterval: 500 * time.Millisecond})
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := inventoryWatch.Run(ctx, queue)
		assert.NoError(t, err)
		wg.Done()
	}()
	cancelFn()
	wg.Wait()
}

func mockState() *cluster.State {
	return &cluster.State{
		Cluster:       &model.ClusterEntity{},
		Configuration: &model.ClusterConfigurationEntity{},
		Status: &model.ClusterStatusEntity{
			Cluster: "foo",
		},
	}
}
