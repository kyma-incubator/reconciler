package reconciliation

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
)

type InMemoryReconciliationRepository struct {
	reconciliations map[string]*model.ReconciliationEntity       //key: clusterName
	operations      map[string]map[string]*model.OperationEntity //key1:schedulingID, key2:correlationID
	status          map[int64]*model.ClusterStatusEntity         //key1:schedulingID, key2:correlationID
	mu              sync.Mutex
}

func (r *InMemoryReconciliationRepository) EnableDebugLogging(schedulingID string, correlationID ...string) error {

	recon, err := r.GetReconciliation(schedulingID)
	if err != nil {
		return err
	}
	if recon.Status.IsFinal() {
		return fmt.Errorf("could not change debug log for reconciliation (schedulingID=%s): reconciliation state is final: %s", schedulingID, recon.Status)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.operations[schedulingID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}

	if len(correlationID) == 1 {

		op, ok := r.operations[schedulingID][correlationID[0]]
		if !ok {
			return &repository.EntityNotFoundError{}
		}

		// copy the operation to avoid having data races while writing
		opCopy := *op

		if opCopy.State != model.OperationStateNew {
			return fmt.Errorf("reconRepo failed to enable debug log level for requested operations")
		}
		opCopy.Debug = true

		opCopy.Updated = time.Now().UTC()

		r.operations[schedulingID][correlationID[0]] = &opCopy

		return nil
	}

	for corrID, op := range r.operations[schedulingID] {

		// copy the operation to avoid having data races while writing
		opCopy := *op
		if opCopy.State != model.OperationStateNew {
			continue
		}
		opCopy.Debug = true

		opCopy.Updated = time.Now().UTC()

		r.operations[schedulingID][corrID] = &opCopy
	}

	return nil
}

func NewInMemoryReconciliationRepository() Repository {
	return &InMemoryReconciliationRepository{
		reconciliations: make(map[string]*model.ReconciliationEntity),
		operations:      make(map[string]map[string]*model.OperationEntity),
		status:          make(map[int64]*model.ClusterStatusEntity),
	}
}

func (r *InMemoryReconciliationRepository) WithTx(_ *db.TxConnection) (Repository, error) {
	return r, nil
}

func (r *InMemoryReconciliationRepository) CreateReconciliation(state *cluster.State, cfg *model.ReconciliationSequenceConfig) (*model.ReconciliationEntity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existingRecon, ok := r.reconciliations[state.Cluster.RuntimeID]; ok {
		return nil, &DuplicateClusterReconciliationError{
			cluster:      existingRecon.RuntimeID,
			schedulingID: existingRecon.SchedulingID,
		}
	}

	//create reconciliation
	reconEntity := &model.ReconciliationEntity{
		Lock:                state.Cluster.RuntimeID,
		RuntimeID:           state.Cluster.RuntimeID,
		ClusterConfig:       state.Configuration.Version,
		ClusterConfigStatus: state.Status.ID,
		SchedulingID:        fmt.Sprintf("%s--%s", state.Cluster.RuntimeID, uuid.NewString()),
		Created:             time.Now().UTC(),
	}
	r.reconciliations[state.Cluster.RuntimeID] = reconEntity

	if _, ok := r.operations[reconEntity.SchedulingID]; !ok {
		r.operations[reconEntity.SchedulingID] = make(map[string]*model.OperationEntity)
	}

	opType := model.OperationTypeReconcile
	if state.Status.Status.IsDeletionInProgress() {
		opType = model.OperationTypeDelete
	}

	//get reconciliation sequence
	sequence := state.Configuration.GetReconciliationSequence(cfg)
	for idx, components := range sequence.Queue {
		priority := idx + 1
		for _, component := range components {
			correlationID := fmt.Sprintf("%s--%s", state.Cluster.RuntimeID, uuid.NewString())

			r.operations[reconEntity.SchedulingID][correlationID] = &model.OperationEntity{
				Priority:      int64(priority),
				SchedulingID:  reconEntity.SchedulingID,
				CorrelationID: correlationID,
				RuntimeID:     reconEntity.RuntimeID,
				ClusterConfig: state.Configuration.Version,
				Component:     component.Component,
				State:         model.OperationStateNew,
				Type:          opType,
				Retries:       0,
				RetryID:       uuid.NewString(),
				Created:       time.Now().UTC(),
				Updated:       time.Now().UTC(),
			}
		}
	}

	// cluster statuses
	statusEntity := &model.ClusterStatusEntity{
		ID:             state.Status.ID,
		ClusterVersion: state.Status.ClusterVersion,
		ConfigVersion:  state.Status.ConfigVersion,
		Status:         state.Status.Status,
		Deleted:        state.Status.Deleted,
		RuntimeID:      state.Status.RuntimeID,
		Created:        state.Status.Created,
	}
	r.status[statusEntity.ID] = statusEntity

	return reconEntity, nil
}

func (r *InMemoryReconciliationRepository) RemoveReconciliationBySchedulingID(schedulingID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	removeSchedulingID(schedulingID, r.reconciliations, r.operations)
	return nil
}

func (r *InMemoryReconciliationRepository) RemoveReconciliationsBySchedulingID(schedulingIDs []interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, schedulingID := range schedulingIDs {
		removeSchedulingID(schedulingID.(string), r.reconciliations, r.operations)
	}
	return nil
}

func (r *InMemoryReconciliationRepository) RemoveReconciliationByRuntimeID(runtimeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var schedulingID string
	if r.reconciliations[runtimeID] != nil {
		schedulingID = r.reconciliations[runtimeID].SchedulingID
	}
	delete(r.reconciliations, runtimeID)
	delete(r.operations, schedulingID)

	return nil
}

func (r *InMemoryReconciliationRepository) RemoveReconciliationsBeforeDeadline(runtimeID string, latestSchedulingID string, deadline time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for schedulingID, recon := range r.operations {
		for correlationID, op := range recon {
			if op.RuntimeID == runtimeID && op.SchedulingID != latestSchedulingID && op.Created.Before(deadline) {
				delete(r.operations[schedulingID], correlationID)
			}
		}
		if len(recon) == 0 {
			delete(r.operations, schedulingID)
			delete(r.reconciliations, runtimeID)
		}
	}

	return nil
}

func (r *InMemoryReconciliationRepository) GetRuntimeIDs() ([]string, error) {
	var runtimeIDs []string
	for key := range r.reconciliations {
		runtimeIDs = append(runtimeIDs, key)
	}
	return runtimeIDs, nil
}

func (r *InMemoryReconciliationRepository) GetReconciliation(schedulingID string) (*model.ReconciliationEntity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, recon := range r.reconciliations {
		if recon.SchedulingID == schedulingID {
			return recon, nil
		}
	}
	return nil, &repository.EntityNotFoundError{}
}

func (r *InMemoryReconciliationRepository) FinishReconciliation(schedulingID string, status *model.ClusterStatusEntity) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, recon := range r.reconciliations {
		if recon.SchedulingID == schedulingID {
			if recon.Finished {
				return fmt.Errorf("reconciliation with schedulingID '%s' is already finished", schedulingID)
			}
			recon.Lock = ""
			recon.Finished = true
			recon.ClusterConfigStatus = status.ID
			recon.Status = status.Status
			recon.Updated = time.Now().UTC()
			return nil
		}
	}

	return fmt.Errorf("no reconciliation found with schedulingID '%s': "+
		"cannot finish reconciliation", schedulingID)
}

func (r *InMemoryReconciliationRepository) GetReconciliations(filter Filter) ([]*model.ReconciliationEntity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.getReconciliations(filter)
}

func (r *InMemoryReconciliationRepository) getReconciliations(filter Filter) ([]*model.ReconciliationEntity, error) {
	var result []*model.ReconciliationEntity
	for _, reconciliation := range r.reconciliations {
		if filter != nil && filter.FilterByInstance(reconciliation) == nil {
			continue
		}
		result = append(result, reconciliation)
	}
	return result, nil
}

func (r *InMemoryReconciliationRepository) GetOperations(filter operation.Filter) ([]*model.OperationEntity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result []*model.OperationEntity
	for _, val := range r.operations {
		for _, v := range val {
			if filter != nil && filter.FilterByInstance(v) == nil {
				continue
			}

			result = append(result, v)
		}
	}

	return result, nil
}

func (r *InMemoryReconciliationRepository) GetOperation(schedulingID, correlationID string) (*model.OperationEntity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	operations, ok := r.operations[schedulingID]
	if !ok {
		return nil, nil
	}
	op, ok := operations[correlationID]
	if !ok {
		return nil, nil
	}
	return op, nil
}

func (r *InMemoryReconciliationRepository) GetProcessableOperations(maxParallelOpsPerRecon int) ([]*model.OperationEntity, error) {
	allOps, err := r.GetReconcilingOperations()
	if err != nil {
		return nil, err
	}
	return findProcessableOperations(allOps, maxParallelOpsPerRecon), nil
}

func (r *InMemoryReconciliationRepository) GetReconcilingOperations() ([]*model.OperationEntity, error) {
	var allOps []*model.OperationEntity
	for _, mapOpsByCorrID := range r.operations {
		for _, op := range mapOpsByCorrID {
			allOps = append(allOps, op)
		}
	}
	return allOps, nil
}

func (r *InMemoryReconciliationRepository) UpdateOperationState(schedulingID, correlationID string, state model.OperationState, allowInState bool, reasons ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.operations[schedulingID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}
	op, ok := r.operations[schedulingID][correlationID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}
	if err := operationAlreadyInState(op, state); err != nil && !allowInState {
		return err
	}

	// copy the operation to avoid having data races while writing
	opCopy := *op

	if opCopy.State.IsFinal() {
		return fmt.Errorf("cannot update state of operation for component '%s' (schedulingID:%s/correlationID:'%s) "+
			"to new state '%s' because operation is already in final state '%s'",
			opCopy.Component, opCopy.SchedulingID, opCopy.CorrelationID, state, opCopy.State)
	}

	reason, err := concatStateReasons(state, reasons)
	if err != nil {
		return err
	}

	//update operation
	opCopy.State = state
	opCopy.Reason = reason
	opCopy.Updated = time.Now().UTC()

	r.operations[schedulingID][correlationID] = &opCopy

	return nil
}

func (r *InMemoryReconciliationRepository) UpdateOperationRetryID(schedulingID, correlationID, retryID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.operations[schedulingID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}
	op, ok := r.operations[schedulingID][correlationID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}

	// copy the operation to avoid having data races while writing
	opCopy := *op

	//update operation
	if opCopy.RetryID != retryID {
		opCopy.RetryID = retryID
		opCopy.Retries++
	}

	opCopy.Updated = time.Now().UTC()

	r.operations[schedulingID][correlationID] = &opCopy

	return nil
}

func (r *InMemoryReconciliationRepository) UpdateOperationPickedUp(schedulingID, correlationID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.operations[schedulingID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}
	op, ok := r.operations[schedulingID][correlationID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}

	// copy the operation to avoid having data races while writing
	opCopy := *op

	opCopy.PickedUp = time.Now()
	r.operations[schedulingID][correlationID] = &opCopy

	return nil
}

func (r *InMemoryReconciliationRepository) UpdateComponentOperationProcessingDuration(schedulingID, correlationID string, processingDuration int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.operations[schedulingID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}
	op, ok := r.operations[schedulingID][correlationID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}

	// copy the operation to avoid having data races while writing
	opCopy := *op

	opCopy.ProcessingDuration = int64(processingDuration)
	r.operations[schedulingID][correlationID] = &opCopy

	return nil
}

func (r *InMemoryReconciliationRepository) GetComponentOperationProcessingDuration(component string, state model.OperationState) (int64, error) {
	operations, err := r.GetOperations(&operation.FilterMixer{
		Filters: []operation.Filter{
			&operation.WithComponentName{Component: component},
			&operation.WithStates{
				States: []model.OperationState{state},
			},
			&operation.LimitByLastUpdate{Count: 1},
		},
	})
	if err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(operations) == 0 {

		return 0, nil
	}

	return operations[0].ProcessingDuration, nil
}

func (r *InMemoryReconciliationRepository) GetMothershipOperationProcessingDuration(component string, state model.OperationState, startTime metricStartTime) (int64, error) {
	operations, err := r.GetOperations(&operation.FilterMixer{
		Filters: []operation.Filter{
			&operation.WithComponentName{Component: component},
			&operation.WithStates{
				States: []model.OperationState{state},
			},
			&operation.LimitByLastUpdate{Count: 1},
		},
	})
	if err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(operations) == 0 {
		return 0, nil
	}

	var duration time.Duration
	op := operations[0]

	switch startTime {
	case Created:
		duration = op.Updated.Sub(op.Created)
	case PickedUp:
		duration = op.Updated.Sub(op.PickedUp)
	}
	return duration.Milliseconds(), nil
}

func (r *InMemoryReconciliationRepository) GetAllComponents() ([]string, error) {
	operations, err := r.GetOperations(nil)
	if err != nil {
		return []string{}, err
	}

	var components []string
	for _, op := range operations {
		components = append(components, op.Component)
	}
	components = unique(components)
	return components, nil
}

func unique(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func removeSchedulingID(schedulingID string, reconciliations map[string]*model.ReconciliationEntity, operations map[string]map[string]*model.OperationEntity) {
	for _, recon := range reconciliations {
		if recon.SchedulingID == schedulingID {
			delete(reconciliations, recon.RuntimeID)
			break
		}
	}

	delete(operations, schedulingID)
}
