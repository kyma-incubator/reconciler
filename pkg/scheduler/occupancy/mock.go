package occupancy

import "github.com/kyma-incubator/reconciler/pkg/db"

type MockRepository struct {
	CreateWorkerPoolOccupancyResult string
	UpdateWorkerPoolOccupancyResult error
	GetMeanWorkerPoolOccupancyResult float64
	RemoveWorkerPoolOccupancyResult error
}


func (mr *MockRepository) CreateWorkerPoolOccupancy(poolSize int) (string, error) {
	return mr.CreateWorkerPoolOccupancyResult, nil
}

func (mr *MockRepository) UpdateWorkerPoolOccupancy(poolId string, runningWorkers int) error {
	return mr.UpdateWorkerPoolOccupancyResult
}
func (mr *MockRepository) GetMeanWorkerPoolOccupancy() (float64, error) {
	return mr.GetMeanWorkerPoolOccupancyResult, nil
}

func (mr *MockRepository) RemoveWorkerPoolOccupancy(poolId string) error {
	return mr.RemoveWorkerPoolOccupancyResult
}

func (mr *MockRepository) WithTx(tx *db.TxConnection) (Repository, error) {
	return mr, nil
}
