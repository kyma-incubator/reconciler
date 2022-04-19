package reconciliation

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
	"time"
)

type MockRepository struct {
	CreateReconciliationResult                          *model.ReconciliationEntity
	RemoveReconciliationResult                          error
	RemoveReconciliationRecording                       []string
	GetReconciliationResult                             *model.ReconciliationEntity
	GetReconciliationsResult                            []*model.ReconciliationEntity
	GetReconciliationsCount                             int
	OnGetReconciliations                                func(*MockRepository)
	FinishReconciliationResult                          error
	GetOperationsResult                                 []*model.OperationEntity
	GetOperationResult                                  *model.OperationEntity
	GetProcessableOperationsResult                      []*model.OperationEntity
	GetReconcilingOperationsResult                      []*model.OperationEntity
	UpdateOperationStateResult                          error
	UpdateOperationRetryIDResult                        error
	UpdateOperationPickedUpResult                       error
	UpdateComponentOperationProcessingDurationResult    error
	GetComponentOperationProcessingDurationResult       int64
	GetComponentOperationProcessingDurationResultError  error
	GetMothershipOperationProcessingDurationResult      int64
	GetMothershipOperationProcessingDurationResultError error
	GetAllComponentsResult                              []string
	GetAllComponentsResultError                         error
	EnableDebugLoggingResult                            error
  	GetStatusIDsOlderThanDeadlineResult                 map[int64]bool
}

func (mr *MockRepository) EnableDebugLogging(schedulingID string, correlationID ...string) error {
	return mr.EnableDebugLoggingResult
}

func (mr *MockRepository) CreateReconciliation(state *cluster.State, cfg *model.ReconciliationSequenceConfig) (*model.ReconciliationEntity, error) {
	return mr.CreateReconciliationResult, nil
}

func (mr *MockRepository) RemoveReconciliationBySchedulingID(schedulingID string) error {
	mr.RemoveReconciliationRecording = append(mr.RemoveReconciliationRecording, schedulingID)
	return mr.RemoveReconciliationResult
}

func (mr *MockRepository) RemoveReconciliationByRuntimeID(runtimeID string) error {
	mr.RemoveReconciliationRecording = append(mr.RemoveReconciliationRecording, runtimeID)
	return mr.RemoveReconciliationResult
}

func (mr *MockRepository) RemoveReconciliationsBySchedulingID(schedulingIDs []interface{}) error {
	var schedulingIDsStrings = make([]string, len(schedulingIDs))
	for i, schedulingID := range schedulingIDs {
		schedulingIDsStrings[i] = schedulingID.(string)
	}
	mr.RemoveReconciliationRecording = append(mr.RemoveReconciliationRecording, schedulingIDsStrings...)
	return mr.RemoveReconciliationResult
}

func (mr *MockRepository) RemoveReconciliationsBeforeDeadline(runtimeID string, latestSchedulingID string, deadline time.Time) error {
	for _, recon := range mr.GetReconciliationsResult {
		if recon.RuntimeID == runtimeID && recon.SchedulingID != latestSchedulingID && recon.Created.Before(deadline) {
			mr.RemoveReconciliationRecording = append(mr.RemoveReconciliationRecording, recon.SchedulingID)
		}
	}
	return nil
}

func (mr *MockRepository) RemoveReconciliationsForObsoleteStatus(deadline time.Time) (int, error) {
	delCount := 0
	for _, recon := range mr.GetReconciliationsResult {
		if mr.GetStatusIDsOlderThanDeadlineResult[recon.ClusterConfigStatus] {
			mr.RemoveReconciliationRecording = append(mr.RemoveReconciliationRecording, recon.SchedulingID)
			delCount++
		}
	}
	return delCount, nil
}

func (mr *MockRepository) GetRuntimeIDs() ([]string, error) {
	runtimeIDsCollector := map[string]interface{}{}
	var runtimeIDs []string
	for _, recon := range mr.GetReconciliationsResult {
		if _, ok := runtimeIDsCollector[recon.RuntimeID]; !ok {
			runtimeIDsCollector[recon.RuntimeID] = true
		}
	}
	for key := range runtimeIDsCollector {
		runtimeIDs = append(runtimeIDs, key)
	}
	return runtimeIDs, nil
}

func (mr *MockRepository) GetReconciliation(schedulingID string) (*model.ReconciliationEntity, error) {
	return mr.GetReconciliationResult, nil
}

func (mr *MockRepository) GetReconciliations(filter Filter) ([]*model.ReconciliationEntity, error) {
	res := mr.GetReconciliationsResult
	mr.GetReconciliationsCount++
	if mr.OnGetReconciliations != nil {
		mr.OnGetReconciliations(mr) //update state
	}
	return res, nil
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

func (mr *MockRepository) UpdateComponentOperationProcessingDuration(schedulingID, correlationID string, processingDuration int) error {
	return mr.UpdateComponentOperationProcessingDurationResult
}

func (mr *MockRepository) GetComponentOperationProcessingDuration(component string, state model.OperationState) (int64, error) {
	return mr.GetComponentOperationProcessingDurationResult, mr.GetComponentOperationProcessingDurationResultError
}

func (mr *MockRepository) GetMothershipOperationProcessingDuration(component string, state model.OperationState, startTime metricStartTime) (int64, error) {
	return mr.GetMothershipOperationProcessingDurationResult, mr.GetMothershipOperationProcessingDurationResultError
}

func (mr *MockRepository) GetAllComponents() ([]string, error) {
	return mr.GetAllComponentsResult, mr.GetAllComponentsResultError
}
