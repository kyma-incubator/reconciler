package reconciliation

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type MockRepository struct {
	CreateReconciliationResult     *model.ReconciliationEntity
	RemoveReconciliationResult     error
	GetReconciliationResult        *model.ReconciliationEntity
	GetReconciliationsResult       []*model.ReconciliationEntity
	FinishReconciliationResult     error
	GetOperationsResult            []*model.OperationEntity
	GetOperationResult             *model.OperationEntity
	GetProcessableOperationsResult []*model.OperationEntity
	GetReconcilingOperationsResult []*model.OperationEntity
	UpdateOperationStateResult     error
}

func (mr *MockRepository) CreateReconciliation(state *cluster.State, preComponents [][]string) (*model.ReconciliationEntity, error) {
	return mr.CreateReconciliationResult, nil
}

func (mr *MockRepository) RemoveReconciliation(schedulingID string) error {
	return mr.RemoveReconciliationResult
}

func (mr *MockRepository) GetReconciliation(schedulingID string) (*model.ReconciliationEntity, error) {
	return mr.GetReconciliationResult, nil
}

func (mr *MockRepository) GetReconciliations(filter Filter) ([]*model.ReconciliationEntity, error) {
	return mr.GetReconciliationsResult, nil
}

func (mr *MockRepository) FinishReconciliation(schedulingID string, status *model.ClusterStatusEntity) error {
	return mr.FinishReconciliationResult
}

func (mr *MockRepository) GetOperations(schedulingID string, state ...model.OperationState) ([]*model.OperationEntity, error) {
	return mr.GetOperationsResult, nil
}

func (mr *MockRepository) GetOperation(schedulingID, correlationID string) (*model.OperationEntity, error) {
	return mr.GetOperationResult, nil
}

func (mr *MockRepository) GetProcessableOperations(maxParallelOpsPerRecon int) ([]*model.OperationEntity, error) {
	return mr.GetProcessableOperationsResult, nil
}

func (mr *MockRepository) GetReconcilingOperations() ([]*model.OperationEntity, error) {
	return mr.GetReconcilingOperationsResult, nil
}

func (mr *MockRepository) UpdateOperationState(schedulingID, correlationID string, state model.OperationState, allowInState bool, reason ...string) error {
	return mr.UpdateOperationStateResult
}

func (mr *MockRepository) WithTx(tx *db.TxConnection) (Repository, error) {
	return mr, nil
}

func (mr *MockRepository) CreateWorkerPoolOccupancy(poolSize int) (string, error) {
	//TODO: implement
	return "", nil
}

func (mr *MockRepository) UpdateWorkerPoolOccupancy(poolId string, runningWorkers int) error {
	//TODO: implement
	return nil
}
func (mr *MockRepository) GetMeanWorkerPoolOccupancy() (float64, error) {
	//TODO: implement
	return 0, nil
}

func (mr *MockRepository) RemoveWorkerPoolOccupancy(poolId string) error {
	//TODO: implement
	return nil
}