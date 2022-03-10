package occupancy

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

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
