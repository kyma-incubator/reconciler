package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRemoteScheduler(t *testing.T) {
	state := cluster.State{
		Cluster: &model.ClusterEntity{},
		Configuration: &model.ClusterConfigurationEntity{
			Contract: 1,
			Components: fixUgliness([]keb.Components{
				{Component: "logging"},
				{Component: "monitoring"},
			}),
		},
		Status: &model.ClusterStatusEntity{},
	}

	var queue InventoryQueue
	inventoryWatchStub := &MockInventoryWatcher{}
	inventoryWatchStub.On("Run", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			queue = args.Get(1).(InventoryQueue)
			queue <- state
		})

	workerMock := &MockReconciliationWorker{}
	workerMock.On("Reconcile", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	workerFactoryMock := &MockWorkerFactory{}
	workerFactoryMock.On("ForComponent", "logging").Return(workerMock, nil)
	workerFactoryMock.On("ForComponent", "monitoring").Return(workerMock, nil)

	l, _ := logger.NewLogger(true)
	sut := RemoteScheduler{
		inventoryWatch: inventoryWatchStub,
		workerFactory:  workerFactoryMock,
		poolSize:       2,
		logger:         l,
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	err := sut.Run(ctx)
	require.NoError(t, err)

	workerFactoryMock.AssertNumberOfCalls(t, "ForComponent", 2)
	workerMock.AssertNumberOfCalls(t, "Reconcile", 2)
}
