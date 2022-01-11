package occupancy

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
)

type Repository interface {

	CreateWorkerPoolOccupancy(poolSize int) (string, error)
	UpdateWorkerPoolOccupancy(poolId string, runningWorkers int) error
	GetMeanWorkerPoolOccupancy() (float64, error)
	RemoveWorkerPoolOccupancy(poolId string) error
	WithTx(tx *db.TxConnection) (Repository, error)

}
