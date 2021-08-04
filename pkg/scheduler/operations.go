package scheduler

import (
	"fmt"
	"sync"
	"time"
)

const (
	StateNew        = "New"
	StateInProgress = "InProgress"
	StateDone       = "Done"
	StateError      = "Error"
	StateFailed     = "Failed"
)

type OperationState struct {
	State     string
	Reason    string
	UpdatedAt time.Time
}

type OperationsRegistry interface {
	RegisterOperation(id string) (*OperationState, error)
	GetOperation(id string) *OperationState
	RemoveOperation(id string) error
	SetInProgress(id string) error
	SetDone(id string) error
	SetError(id, reason string) error
	SetFailed(id, reason string) error
}

type DefaultOperationsRegistry struct {
	registry map[string]OperationState
	mu       sync.Mutex
}

func (or *DefaultOperationsRegistry) RegisterOperation(id string) (*OperationState, error) {
	or.mu.Lock()
	defer or.mu.Unlock()

	op, ok := or.registry[id]
	if ok {
		return nil, fmt.Errorf("Operation with the following id %s already registered", id)
	}

	op = OperationState{
		State:     StateNew,
		UpdatedAt: time.Now(),
	}
	or.registry[id] = op
	return &op, nil
}

func (or *DefaultOperationsRegistry) GetOperation(id string) *OperationState {
	or.mu.Lock()
	defer or.mu.Unlock()

	op, ok := or.registry[id]
	if !ok {
		return nil
	}
	return &op
}

func (or *DefaultOperationsRegistry) RemoveOperation(id string) error {
	or.mu.Lock()
	defer or.mu.Unlock()

	_, ok := or.registry[id]
	if !ok {
		return fmt.Errorf("Operation with the following id %s not found", id)
	}
	delete(or.registry, id)
	return nil
}

func (or *DefaultOperationsRegistry) SetInProgress(id string) error {
	return or.update(id, StateInProgress, "")
}

func (or *DefaultOperationsRegistry) SetDone(id string) error {
	return or.update(id, StateDone, "")
}

func (or *DefaultOperationsRegistry) SetError(id, reason string) error {
	return or.update(id, StateError, reason)
}

func (or *DefaultOperationsRegistry) SetFailed(id, reason string) error {
	return or.update(id, StateFailed, reason)
}

func (or *DefaultOperationsRegistry) update(id, state, reason string) error {
	or.mu.Lock()
	defer or.mu.Unlock()

	op, ok := or.registry[id]
	if !ok {
		return fmt.Errorf("Operation with the following id %s not found", id)
	}

	op = OperationState{
		State:     state,
		Reason:    reason,
		UpdatedAt: time.Now(),
	}
	or.registry[id] = op
	return nil
}
