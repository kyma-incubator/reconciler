package occupancy

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
)

type Repository interface {

	CreateWorkerPoolOccupancy(component string, poolSize int) (string, error)
	UpdateWorkerPoolOccupancy(poolId string, runningWorkers int) error
	GetMeanWorkerPoolOccupancy() (float64, error)
	RemoveWorkerPoolOccupancy(poolId string) error
	GetComponentList() ([]string, error)
	GetMeanWorkerPoolOccupancyByComponent(component string) (float64, error)
	WithTx(tx *db.TxConnection) (Repository, error)

}

type ComponentOccupancy struct{
	ComponentName string
	Occupancy float64
}
