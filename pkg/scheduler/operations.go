package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
)

type OperationsRegistry interface {
	GetDoneOperations(schedulingID string) ([]*model.OperationEntity, error)
	RegisterOperation(correlationID, schedulingID, component string, version int64) (*model.OperationEntity, error)
	GetOperation(correlationID, schedulingID string) (*model.OperationEntity, error)
	RemoveOperation(correlationID, schedulingID string) error
	SetInProgress(correlationID, schedulingID string) error
	SetDone(correlationID, schedulingID string) error
	SetError(correlationID, schedulingID, reason string) error
	SetClientError(correlationID, schedulingID, reason string) error
	SetFailed(correlationID, schedulingID, reason string) error
}

type OperationNotFoundError struct {
	schedulingID  string
	correlationID string
}

func (err *OperationNotFoundError) Error() string {
	return fmt.Sprintf("operation with id '%s' not found (schedulingID:%s/correlationID:%s)", err.correlationID,
		err.schedulingID, err.correlationID)
}

func newOperationNotFoundError(schedulingID string, correlationID string) error {
	return &OperationNotFoundError{
		schedulingID:  schedulingID,
		correlationID: correlationID,
	}
}

func IsOperationNotFoundError(err error) bool {
	_, ok := err.(*OperationNotFoundError)
	return ok
}

type PersistedOperationsRegistry struct {
	*repository.Repository
}

func NewPersistedOperationsRegistry(dbFac db.ConnectionFactory, debug bool) (OperationsRegistry, error) {
	repo, err := repository.NewRepository(dbFac, debug)
	if err != nil {
		return nil, err
	}
	return &PersistedOperationsRegistry{repo}, nil
}

func (or *PersistedOperationsRegistry) GetDoneOperations(schedulingID string) ([]*model.OperationEntity, error) {
	return nil, nil
}

func (or *PersistedOperationsRegistry) RegisterOperation(correlationID, schedulingID, component string, version int64) (*model.OperationEntity, error) {
	dbOps := func() (interface{}, error) {
		opEntity := &model.OperationEntity{
			SchedulingID:  schedulingID,
			CorrelationID: correlationID,
			ConfigVersion: version,
			Component:     component,
			State:         model.OperationStateNew,
		}
		_, err := or.GetOperation(correlationID, schedulingID)
		if err == nil {
			return nil, fmt.Errorf("operation with the following id %s already registered", correlationID)
		} else if !repository.IsNotFoundError(err) {
			//unexpected error
			return nil, err
		} else {
			q, err := db.NewQuery(or.Conn, opEntity)
			if err != nil {
				return nil, err
			}
			err = q.Insert().Exec()
			if err != nil {
				return nil, err
			}
			return opEntity, nil
		}
	}
	entity, err := db.TransactionResult(or.Conn, dbOps, or.Logger)
	if err != nil {
		return nil, err
	}
	return entity.(*model.OperationEntity), nil
}

func (or *PersistedOperationsRegistry) GetOperation(correlationID, schedulingID string) (*model.OperationEntity, error) {
	q, err := db.NewQuery(or.Conn, &model.OperationEntity{})
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"CorrelationID": correlationID,
		"SchedulingID":  schedulingID,
	}
	opEntity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, or.NewNotFoundError(err, opEntity, whereCond)
	}
	return opEntity.(*model.OperationEntity), nil
}

func (or *PersistedOperationsRegistry) RemoveOperation(correlationID, schedulingID string) error {
	dbOps := func() (interface{}, error) {
		_, err := or.GetOperation(correlationID, schedulingID)
		if err != nil {
			if !repository.IsNotFoundError(err) {
				return nil, err
			}
			return nil, fmt.Errorf("operation with the following id %s not found", correlationID)
		}

		q, err := db.NewQuery(or.Conn, &model.OperationEntity{})
		if err != nil {
			return nil, err
		}
		whereCond := map[string]interface{}{
			"CorrelationID": correlationID,
			"SchedulingID":  schedulingID,
		}
		_, err = q.Delete().
			Where(whereCond).
			Exec()
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	_, err := db.TransactionResult(or.Conn, dbOps, or.Logger)
	if err != nil {
		return err
	}
	return nil
}

func (or *PersistedOperationsRegistry) SetInProgress(correlationID, schedulingID string) error {
	return or.updateState(correlationID, schedulingID, model.OperationStateInProgress, "")
}

func (or *PersistedOperationsRegistry) SetDone(correlationID, schedulingID string) error {
	return or.updateState(correlationID, schedulingID, model.OperationStateDone, "")
}

func (or *PersistedOperationsRegistry) SetError(correlationID, schedulingID, reason string) error {
	return or.updateState(correlationID, schedulingID, model.OperationStateError, reason)
}

func (or *PersistedOperationsRegistry) SetClientError(correlationID, schedulingID, reason string) error {
	return or.updateState(correlationID, schedulingID, model.OperationStateClientError, reason)
}

func (or *PersistedOperationsRegistry) SetFailed(correlationID, schedulingID, reason string) error {
	return or.updateState(correlationID, schedulingID, model.OperationStateFailed, reason)
}

func (or *PersistedOperationsRegistry) updateState(correlationID, schedulingID, state, reason string) error {
	dbOps := func() (interface{}, error) {
		op, err := or.GetOperation(correlationID, schedulingID)
		if err != nil {
			if !repository.IsNotFoundError(err) {
				return nil, err
			}
			return nil, fmt.Errorf("operation with the following id %s not found", correlationID)
		}

		q, err := db.NewQuery(or.Conn, &model.OperationEntity{
			SchedulingID:  op.SchedulingID,
			CorrelationID: op.CorrelationID,
			ConfigVersion: op.ConfigVersion,
			Component:     op.Component,
			State:         model.OperationState(state),
			Reason:        reason,
			Updated:       time.Now(),
		})
		if err != nil {
			return nil, err
		}
		whereCond := map[string]interface{}{
			"CorrelationID": correlationID,
			"SchedulingID":  schedulingID,
		}
		err = q.Update().
			Where(whereCond).
			Exec()
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	_, err := db.TransactionResult(or.Conn, dbOps, or.Logger)
	if err != nil {
		return err
	}
	return nil
}

type InMemoryOperationsRegistry struct {
	registry map[string]map[string]model.OperationEntity
	mu       sync.Mutex
}

func NewInMemoryOperationsRegistry() *InMemoryOperationsRegistry {
	return &InMemoryOperationsRegistry{
		registry: make(map[string]map[string]model.OperationEntity),
	}
}

func (or *InMemoryOperationsRegistry) GetDoneOperations(schedulingID string) ([]*model.OperationEntity, error) {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if !ok {
		return nil, fmt.Errorf("no operations found for scheduling id '%s'", schedulingID)
	}
	var result []*model.OperationEntity
	for idx := range operations {
		op := operations[idx]
		if op.State == model.OperationStateDone {
			result = append(result, &op)
		}
	}
	return result, nil
}

func (or *InMemoryOperationsRegistry) RegisterOperation(correlationID, schedulingID, component string, version int64) (*model.OperationEntity, error) {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if ok {
		_, ok := operations[correlationID]
		if ok {
			return nil, fmt.Errorf("operation with id '%s' already registered", correlationID)
		}
	} else {
		or.registry[schedulingID] = make(map[string]model.OperationEntity)
	}

	op := model.OperationEntity{
		SchedulingID:  schedulingID,
		CorrelationID: correlationID,
		ConfigVersion: version,
		Component:     component,
		State:         model.OperationStateNew,
		Created:       time.Now(),
		Updated:       time.Now(),
	}
	or.registry[schedulingID][correlationID] = op
	return &op, nil
}

func (or *InMemoryOperationsRegistry) GetOperation(correlationID, schedulingID string) (*model.OperationEntity, error) {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if !ok {
		return nil, nil
	}
	op, ok := operations[correlationID]
	if !ok {
		return nil, nil
	}
	return &op, nil
}

func (or *InMemoryOperationsRegistry) RemoveOperation(correlationID, schedulingID string) error {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if !ok {
		return newOperationNotFoundError(schedulingID, correlationID)
	}
	_, ok = operations[correlationID]
	if !ok {
		return newOperationNotFoundError(schedulingID, correlationID)
	}
	delete(or.registry[schedulingID], correlationID)
	return nil
}

func (or *InMemoryOperationsRegistry) SetInProgress(correlationID, schedulingID string) error {
	return or.update(correlationID, schedulingID, model.OperationStateInProgress, "")
}

func (or *InMemoryOperationsRegistry) SetDone(correlationID, schedulingID string) error {
	return or.update(correlationID, schedulingID, model.OperationStateDone, "")
}

func (or *InMemoryOperationsRegistry) SetError(correlationID, schedulingID, reason string) error {
	return or.update(correlationID, schedulingID, model.OperationStateError, reason)
}

func (or *InMemoryOperationsRegistry) SetClientError(correlationID, schedulingID, reason string) error {
	return or.update(correlationID, schedulingID, model.OperationStateClientError, reason)
}

func (or *InMemoryOperationsRegistry) SetFailed(correlationID, schedulingID, reason string) error {
	return or.update(correlationID, schedulingID, model.OperationStateFailed, reason)
}

func (or *InMemoryOperationsRegistry) update(correlationID, schedulingID, state, reason string) error {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if !ok {
		return newOperationNotFoundError(schedulingID, correlationID)
	}
	op, ok := operations[correlationID]
	if !ok {
		return newOperationNotFoundError(schedulingID, correlationID)
	}

	or.registry[schedulingID][correlationID] = model.OperationEntity{
		CorrelationID: correlationID,
		SchedulingID:  schedulingID,
		ConfigVersion: op.ConfigVersion,
		Component:     op.Component,
		State:         model.OperationState(state),
		Reason:        reason,
		Created:       op.Created,
		Updated:       time.Now(),
	}
	return nil
}
