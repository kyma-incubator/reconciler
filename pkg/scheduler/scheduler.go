package scheduler

import (
	"context"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/panjf2000/ants/v2"
)

const poolCap int = 10

type Scheduler interface {
	Run(ctx context.Context) error
}

type RemoteScheduler struct {
	inventoryWatch InventoryWatcher
}

func NewRemoteScheduler(inventoryWatch InventoryWatcher) (Scheduler, error) {
	return &RemoteScheduler{inventoryWatch}, nil
}

func (rs *RemoteScheduler) Run(ctx context.Context) error {
	queue := make(chan cluster.State, poolCap)

	workersPool, err := ants.NewPoolWithFunc(poolCap, func(i interface{}) {
		rs.Worker(i.(cluster.State))
	})
	if err != nil {
		return err
	}

	go rs.inventoryWatch.Run(ctx, queue)

loop:
	for {
		select {
		case cluster := <-queue:
			go workersPool.Invoke(cluster)
		case <-ctx.Done():
			break loop
		}
	}
	return nil
}

func (rs *RemoteScheduler) Worker(cluster cluster.State) {

}

// func NewLocalScheduler() (Scheduler, error) {
// 	return &LocalScheduler{}, nil
// }

// type LocalScheduler struct{}

// func (ls *LocalScheduler) Run() {

// }
