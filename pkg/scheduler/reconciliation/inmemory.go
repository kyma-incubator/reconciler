package reconciliation

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"sync"
	"time"

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

	if existingRecon, ok := r.reconciliations[state.Cluster.Cluster]; ok {
		return nil, &DuplicateClusterReconciliationError{
			cluster:      existingRecon.Cluster,
			schedulingID: existingRecon.SchedulingID,
		}
	}

	//create reconciliation
	reconEntity := &model.ReconciliationEntity{
		Lock:                state.Cluster.Cluster,
		Cluster:             state.Cluster.Cluster,
		ClusterConfig:       state.Configuration.Version,
		ClusterConfigStatus: state.Status.ID,
		SchedulingID:        uuid.NewString(),
	}
	r.reconciliations[state.Cluster.Cluster] = reconEntity

	//create operations
	reconSeq, err := state.Configuration.GetReconciliationSequence(preComponents)
	if err != nil {
		return nil, err
	}
	for idx, components := range reconSeq.Queue {
		priority := idx + 1
		for _, component := range components {
			correlationID := uuid.NewString()

			if _, ok := r.operations[reconEntity.SchedulingID]; !ok {
				r.operations[reconEntity.SchedulingID] = make(map[string]*model.OperationEntity)
			}

			r.operations[reconEntity.SchedulingID][correlationID] = &model.OperationEntity{
				Priority:      int64(priority),
				SchedulingID:  reconEntity.SchedulingID,
				CorrelationID: correlationID,
				Cluster:       reconEntity.Cluster,
				ClusterConfig: state.Configuration.Version,
				Component:     component.Component,
				State:         model.OperationStateNew,
				Created:       time.Now(),
				Updated:       time.Now(),
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
			delete(r.reconciliations, recon.Cluster)
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
			recon.Updated = time.Now()
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

func (r *InMemoryReconciliationRepository) GetProcessableOperations() ([]*model.OperationEntity, error) {
	allOps, err := r.GetReconcilingOperations()
	if err != nil {
		return nil, err
	}
	return findProcessableOperations(allOps), nil
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

	operations, ok := r.operations[schedulingID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}
	op, ok := operations[correlationID]
	if !ok {
		return &repository.EntityNotFoundError{}
	}

	if op.State == model.OperationStateDone || op.State == model.OperationStateError {
		return fmt.Errorf("cannot update state of operation for component '%s' (schedulingID:%s/correlationID:'%s) "+
			"to new state '%s' because operation is already in final state '%s'",
			op.Component, op.SchedulingID, op.CorrelationID, state, op.State)
	}

	if op.State == state {
		return newRedundantOperationStateUpdateError(op)
	}

	reason, err := concatStateReasons(state, reasons)
	if err != nil {
		return err
	}
	r.operations[schedulingID][correlationID] = &model.OperationEntity{
		CorrelationID: correlationID,
		SchedulingID:  schedulingID,
		ClusterConfig: op.ClusterConfig,
		Component:     op.Component,
		State:         model.OperationState(state),
		Reason:        reason,
		Created:       op.Created,
		Updated:       time.Now(),
	}

	return nil
}
