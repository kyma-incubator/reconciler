package reconciliation

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
)

type MockRepository struct {
	CreateReconciliationResult                              *model.ReconciliationEntity
	RemoveReconciliationResult                              error
	GetReconciliationResult                                 *model.ReconciliationEntity
	GetReconciliationsResult                                []*model.ReconciliationEntity
	FinishReconciliationResult                              error
	GetOperationsResult                                     []*model.OperationEntity
	GetOperationResult                                      *model.OperationEntity
	GetProcessableOperationsResult                          []*model.OperationEntity
	GetReconcilingOperationsResult                          []*model.OperationEntity
	UpdateOperationStateResult                              error
	UpdateOperationRetryIDResult                            error
	UpdateOperationPickedUpResult                           error
	UpdateComponentOperationProcessingDurationResult        error
	GetMeanComponentOperationProcessingDurationResult       int64
	GetMeanComponentOperationProcessingDurationResultError  error
	GetMeanMothershipOperationProcessingDurationResult      int64
	GetMeanMothershipOperationProcessingDurationResultError error
}

func (mr *MockRepository) CreateReconciliation(state *cluster.State, cfg *model.ReconciliationSequenceConfig) (*model.ReconciliationEntity, error) {
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

func (mr *MockRepository) GetOperations(filters operation.Filter) ([]*model.OperationEntity, error) {
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

func (mr *MockRepository) UpdateOperationRetryID(schedulingID, correlationID, retryID string) error {
	return mr.UpdateOperationRetryIDResult
}

func (mr *MockRepository) UpdateOperationPickedUp(schedulingID, correlationID string) error {
	return mr.UpdateOperationPickedUpResult
}

func (mr *MockRepository) UpdateComponentOperationProcessingDuration(schedulingID, correlationID string, processingDuration int64) error {
	return mr.UpdateComponentOperationProcessingDurationResult
}

func (mr *MockRepository) GetMeanComponentOperationProcessingDuration(component string, state model.OperationState) (int64, error) {
	return mr.GetMeanComponentOperationProcessingDurationResult, r.GetMeanComponentOperationProcessingDurationResultError
}

func (mr *MockRepository) GetMeanMothershipOperationProcessingDuration(component string, state model.OperationState, startTime metricStartTime) (int64, error) {
	return mr.GetMeanMothershipOperationProcessingDurationResult, r.GetMeanMothershipOperationProcessingDurationResultError
}
