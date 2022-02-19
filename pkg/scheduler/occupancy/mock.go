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
	GetComponentIDsResult                       []string
	GetMeanWorkerPoolOccupancyByComponentResult float64
	GetWorkerPoolOccupanciesResult              []*model.WorkerPoolOccupancyEntity
	FindWorkerPoolOccupancyByIDResult           *model.WorkerPoolOccupancyEntity
}

func (mr *MockRepository) GetComponentIDs() ([]string, error) {
	return mr.GetComponentIDsResult, nil
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

func (mr *MockRepository) CreateWorkerPoolOccupancy(poolID, component string, runningWorkers, poolSize int) (*model.WorkerPoolOccupancyEntity, error) {
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

func (mr *MockRepository) CreateOrUpdateWorkerPoolOccupancy(poolID, component string, runningWorkers, poolSize int) (bool, error) {
	if mr.CreateWorkerPoolOccupancyResult != nil {
		return false, nil
	}
	return true, nil
}

func (mr *MockRepository) WithTx(tx *db.TxConnection) (Repository, error) {
	return mr, nil
}

func CreateMockRepository() Repository {
	return &MockRepository{
		CreateWorkerPoolOccupancyResult:   &model.WorkerPoolOccupancyEntity{},
		UpdateWorkerPoolOccupancyResult:   nil,
		RemoveWorkerPoolOccupancyResult:   nil,
		GetComponentListResult:            []string{"mothership"},
		GetWorkerPoolOccupanciesResult:    nil,
		FindWorkerPoolOccupancyByIDResult: &model.WorkerPoolOccupancyEntity{},
	}
}
