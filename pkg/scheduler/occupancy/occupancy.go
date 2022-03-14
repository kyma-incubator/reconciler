package occupancy

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type Observer interface {
	UpdateOccupancy() error
}

type Subject interface {
	RunningWorkers() (int, error)
	Size() int
	RegisterObserver(observer Observer)
	UnregisterObserver(observer Observer)
	Notify() error
}

// Repository There is no In-Memory implementation for this repository,
// as the occupancy tracking is only relevant when the mothership is deployed in a k8s cluster
type Repository interface {
	CreateWorkerPoolOccupancy(poolID, component string, runningWorkers, poolSize int) (*model.WorkerPoolOccupancyEntity, error)
	FindWorkerPoolOccupancyByID(poolID string) (*model.WorkerPoolOccupancyEntity, error)
	GetComponentList() ([]string, error)
	GetWorkerPoolIDs() ([]string, error)
	GetMeanWorkerPoolOccupancyByComponent(component string) (float64, error)
	GetWorkerPoolOccupancies() ([]*model.WorkerPoolOccupancyEntity, error)
	RemoveWorkerPoolOccupancy(poolID string) error
	RemoveWorkerPoolOccupancies(poolIDs []string) (int, error)
	UpdateWorkerPoolOccupancy(poolID string, runningWorkers int) error
	CreateOrUpdateWorkerPoolOccupancy(poolID, component string, runningWorkers, poolSize int) (bool, error)
	WithTx(tx *db.TxConnection) (Repository, error)
}
