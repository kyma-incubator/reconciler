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
	RegisterOperation(operationID, schedulingID, component string) (*OperationState, error)
	GetOperation(operationID, schedulingID string) *OperationState
	RemoveOperation(operationID, schedulingID string) error
	SetInProgress(operationID, schedulingID string) error
	SetDone(operationID, schedulingID string) error
	SetError(operationID, schedulingID, reason string) error
	SetClientError(operationID, schedulingID, reason string) error
	SetFailed(operationID, schedulingID, reason string) error
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
	operations, ok := or.registry[schedulingID]
	if !ok {
		return nil, fmt.Errorf("No operations found for scheduling id %s", schedulingID)
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

func (or *DefaultOperationsRegistry) RegisterOperation(operationID, schedulingID, component string) (*OperationState, error) {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if ok {
		_, ok := operations[operationID]
		if ok {
			return nil, fmt.Errorf("Operation with the following id %s already registered", operationID)
		}
	} else {
		or.registry[schedulingID] = make(map[string]OperationState)
	}

	op := OperationState{
		ID:        operationID,
		Component: component,
		State:     StateNew,
		UpdatedAt: time.Now(),
	}
	or.registry[schedulingID][operationID] = op
	return &op, nil
}

func (or *DefaultOperationsRegistry) GetOperation(operationID, schedulingID string) *OperationState {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if !ok {
		return nil
	}
	op, ok := operations[operationID]
	if !ok {
		return nil
	}
	return &op
}

func (or *DefaultOperationsRegistry) RemoveOperation(operationID, schedulingID string) error {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if !ok {
		return fmt.Errorf("Operation with the following id %s not found", operationID)
	}
	_, ok = operations[operationID]
	if !ok {
		return fmt.Errorf("Operation with the following id %s not found", operationID)
	}
	delete(or.registry[schedulingID], operationID)
	return nil
}

func (or *DefaultOperationsRegistry) SetInProgress(operationID, schedulingID string) error {
	return or.update(operationID, schedulingID, StateInProgress, "")
}

func (or *DefaultOperationsRegistry) SetDone(operationID, schedulingID string) error {
	return or.update(operationID, schedulingID, StateDone, "")
}

func (or *DefaultOperationsRegistry) SetError(operationID, schedulingID, reason string) error {
	return or.update(operationID, schedulingID, StateError, reason)
}

func (or *DefaultOperationsRegistry) SetClientError(operationID, schedulingID, reason string) error {
	return or.update(operationID, schedulingID, StateClientError, reason)
}

func (or *DefaultOperationsRegistry) SetFailed(operationID, schedulingID, reason string) error {
	return or.update(operationID, schedulingID, StateFailed, reason)
}

func (or *DefaultOperationsRegistry) update(operationID, schedulingID, state, reason string) error {
	or.mu.Lock()
	defer or.mu.Unlock()

	operations, ok := or.registry[schedulingID]
	if !ok {
		return fmt.Errorf("Operation with the following id %s not found", operationID)
	}
	op, ok := operations[operationID]
	if !ok {
		return fmt.Errorf("Operation with the following id %s not found", operationID)
	}

	or.registry[schedulingID][operationID] = OperationState{
		ID:        operationID,
		Component: op.Component,
		State:     state,
		Reason:    reason,
		UpdatedAt: time.Now(),
	}
	return nil
}
