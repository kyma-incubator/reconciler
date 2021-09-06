package scheduler

import (
	"fmt"
	"sync"
	"time"
)

const (
	StateNew         = "New"
	StateInProgress  = "InProgress"
	StateDone        = "Done"
	StateClientError = "ClientError"
	StateError       = "Error"
	StateFailed      = "Failed"
)

type OperationState struct {
	ID        string
	Component string
	State     string
	Reason    string
	UpdatedAt time.Time
}

type OperationsRegistry interface {
	GetDoneOperations(schedulingID string) ([]*OperationState, error)
	RegisterOperation(correlationID, schedulingID, component string) (*OperationState, error)
	GetOperation(correlationID, schedulingID string) *OperationState
	RemoveOperation(correlationID, schedulingID string) error
	SetInProgress(correlationID, schedulingID string) error
	SetDone(correlationID, schedulingID string) error
	SetError(correlationID, schedulingID, reason string) error
	SetClientError(correlationID, schedulingID, reason string) error
	SetFailed(correlationID, schedulingID, reason string) error
}

type OperationNowFoundError struct {
	schedulingID  string
	correlationID string
}

func (err *OperationNowFoundError) Error() string {
	return fmt.Sprintf("operation with id '%s' not found (schedulingID:%s/correlationID:%s)", err.correlationID,
		err.schedulingID, err.correlationID)
}

func newOperationNotFoundError(schedulingID string, correlationID string) error {
	return &OperationNowFoundError{
		schedulingID:  schedulingID,
		correlationID: correlationID,
	}
}

func IsOperationNotFoundError(err error) bool {
	_, ok := err.(*OperationNowFoundError)
	return ok
}

type DefaultOperationsRegistry struct {
	registry map[string]map[string]OperationState
	mu       sync.Mutex
}

func NewDefaultOperationsRegistry() *DefaultOperationsRegistry {
	return &DefaultOperationsRegistry{
		registry: make(map[string]map[string]OperationState),
	}
}

func (or *DefaultOperationsRegistry) GetDoneOperations(schedulingID string) ([]*OperationState, error) {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if !ok {
		return nil, fmt.Errorf("no operations found for scheduling id '%s'", schedulingID)
	}
	var result []*OperationState
	for idx := range operations {
		op := operations[idx]
		if op.State == StateDone {
			result = append(result, &op)
		}
	}
	return result, nil
}

func (or *DefaultOperationsRegistry) RegisterOperation(correlationID, schedulingID, component string) (*OperationState, error) {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if ok {
		_, ok := operations[correlationID]
		if ok {
			return nil, fmt.Errorf("operation with id '%s' already registered", correlationID)
		}
	} else {
		or.registry[schedulingID] = make(map[string]OperationState)
	}

	op := OperationState{
		ID:        correlationID,
		Component: component,
		State:     StateNew,
		UpdatedAt: time.Now(),
	}
	or.registry[schedulingID][correlationID] = op
	return &op, nil
}

func (or *DefaultOperationsRegistry) GetOperation(correlationID, schedulingID string) *OperationState {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if !ok {
		return nil
	}
	op, ok := operations[correlationID]
	if !ok {
		return nil
	}
	return &op
}

func (or *DefaultOperationsRegistry) RemoveOperation(correlationID, schedulingID string) error {
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

func (or *DefaultOperationsRegistry) SetInProgress(correlationID, schedulingID string) error {
	return or.update(correlationID, schedulingID, StateInProgress, "")
}

func (or *DefaultOperationsRegistry) SetDone(correlationID, schedulingID string) error {
	return or.update(correlationID, schedulingID, StateDone, "")
}

func (or *DefaultOperationsRegistry) SetError(correlationID, schedulingID, reason string) error {
	return or.update(correlationID, schedulingID, StateError, reason)
}

func (or *DefaultOperationsRegistry) SetClientError(correlationID, schedulingID, reason string) error {
	return or.update(correlationID, schedulingID, StateClientError, reason)
}

func (or *DefaultOperationsRegistry) SetFailed(correlationID, schedulingID, reason string) error {
	return or.update(correlationID, schedulingID, StateFailed, reason)
}

func (or *DefaultOperationsRegistry) update(correlationID, schedulingID, state, reason string) error {
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

	or.registry[schedulingID][correlationID] = OperationState{
		ID:        correlationID,
		Component: op.Component,
		State:     state,
		Reason:    reason,
		UpdatedAt: time.Now(),
	}
	return nil
}
