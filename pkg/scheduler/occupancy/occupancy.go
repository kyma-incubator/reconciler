package occupancy

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type Repository interface {
	CreateWorkerPoolOccupancy(poolID, component string, poolSize int) error
	UpdateWorkerPoolOccupancy(poolID string, runningWorkers int) error
	GetMeanWorkerPoolOccupancy() (float64, error)
	RemoveWorkerPoolOccupancy(poolID string) error
	GetComponentList() ([]string, error)
	GetMeanWorkerPoolOccupancyByComponent(component string) (float64, error)
	WithTx(tx *db.TxConnection) (Repository, error)
	FindWorkerPoolOccupancyByID(poolID string) (*model.WorkerPoolOccupancyEntity, error)
}
