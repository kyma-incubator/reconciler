package reconciliation

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/repository"

	"github.com/kyma-incubator/reconciler/pkg/model"
)

type InMemoryReconciliationRepository struct {
	reconciliations map[string]*model.ReconciliationEntity       //key: clusterName
	operations      map[string]map[string]*model.OperationEntity //key1:schedulingID, key2:correlationID
	mu              sync.Mutex
}

func NewInMemoryReconciliationRepository() Repository {
	return &InMemoryReconciliationRepository{
		reconciliations: make(map[string]*model.ReconciliationEntity),
		operations:      make(map[string]map[string]*model.OperationEntity),
	}
}

func (r *InMemoryReconciliationRepository) CreateReconciliation(state *cluster.State, preComponents []string) (*model.ReconciliationEntity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(state.Configuration.Components) == 0 {
		return nil, newEmptyComponentsReconciliationError(state)
	}

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
	}
	r.reconciliations[state.Cluster.RuntimeID] = reconEntity

	//create operations
	reconSeq := state.Configuration.GetReconciliationSequence(preComponents)

	if _, ok := r.operations[reconEntity.SchedulingID]; !ok {
		r.operations[reconEntity.SchedulingID] = make(map[string]*model.OperationEntity)
	}

	opType := model.OperationTypeReconcile
	if state.Status.Status.IsDeletion() {
		opType = model.OperationTypeDelete
	}

	for idx, components := range reconSeq.Queue {
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
				Created:       time.Now().UTC(),
				Updated:       time.Now().UTC(),
			}
		}
	}

	return reconEntity, nil
}

func (r *InMemoryReconciliationRepository) RemoveReconciliation(schedulingID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, recon := range r.reconciliations {
		if recon.SchedulingID == schedulingID {
			delete(r.reconciliations, recon.RuntimeID)
			break
		}
	}
	delete(r.operations, schedulingID)
	return nil
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

	var result []*model.ReconciliationEntity
	for _, reconciliation := range r.reconciliations {
		if filter != nil && filter.FilterByInstance(reconciliation) == nil {
			continue
		}
		result = append(result, reconciliation)
	}
	return result, nil
}

func (r *InMemoryReconciliationRepository) GetOperations(schedulingID string, states ...model.OperationState) ([]*model.OperationEntity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ops := r.operations[schedulingID]
	var result []*model.OperationEntity
	for _, op := range ops {
		if len(states) > 0 {
			for _, state := range states {
				if op.State == state {
					result = append(result, op)
					break //break state loop
				}
			}
			continue //continue with next operation
		}
		result = append(result, op)
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

func (r *InMemoryReconciliationRepository) UpdateOperationState(schedulingID, correlationID string, state model.OperationState, reasons ...string) error {
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
