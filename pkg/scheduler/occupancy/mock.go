package occupancy

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type MockRepository struct {
	CreateWorkerPoolOccupancyResult             *model.WorkerPoolOccupancyEntity
	UpdateWorkerPoolOccupancyResult             error
	RemoveWorkerPoolOccupancyResult             error
	GetComponentListResult                      []string
	GetMeanWorkerPoolOccupancyByComponentResult float64
	GetWorkerPoolOccupanciesResult              []*model.WorkerPoolOccupancyEntity
	FindWorkerPoolOccupancyByIDResult           *model.WorkerPoolOccupancyEntity
}

func (mr *MockRepository) GetComponentList() ([]string, error) {
	return mr.GetComponentListResult, nil
}

func (mr *MockRepository) GetMeanWorkerPoolOccupancyByComponent(component string) (float64, error) {
	return mr.GetMeanWorkerPoolOccupancyByComponentResult, nil
}

func (mr *MockRepository) FindWorkerPoolOccupancyByID(poolID string) (*model.WorkerPoolOccupancyEntity, error) {
	return mr.FindWorkerPoolOccupancyByIDResult, nil
}

func (mr *MockRepository) CreateWorkerPoolOccupancy(poolID, component string, poolSize int) (*model.WorkerPoolOccupancyEntity, error) {
	return mr.CreateWorkerPoolOccupancyResult, nil
}

func (mr *MockRepository) UpdateWorkerPoolOccupancy(poolID string, runningWorkers int) error {
	return mr.UpdateWorkerPoolOccupancyResult
}
func (mr *MockRepository) GetWorkerPoolOccupancies() ([]*model.WorkerPoolOccupancyEntity, error) {
	return mr.GetWorkerPoolOccupanciesResult, nil
}

func (mr *MockRepository) RemoveWorkerPoolOccupancy(poolID string) error {
	return mr.RemoveWorkerPoolOccupancyResult
}

func (mr *MockRepository) WithTx(tx *db.TxConnection) (Repository, error) {
	return mr, nil
}
