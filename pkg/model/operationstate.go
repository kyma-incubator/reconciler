package model

import (
	"fmt"
	"strings"
)

type OperationState string

const (
	OperationStateNew         OperationState = "new"
	OperationStateInProgress  OperationState = "in_progress"
	OperationStateDone        OperationState = "done"
	OperationStateClientError OperationState = "client_error"
	OperationStateError       OperationState = "error"
	OperationStateFailed      OperationState = "failed"
	OperationStateOrphan      OperationState = "orphan"
)

func NewOperationState(state string) (OperationState, error) {
	var result OperationState
	switch strings.ToLower(state) {
	case string(OperationStateNew):
		result = OperationStateNew
	case string(OperationStateInProgress):
		result = OperationStateInProgress
	case string(OperationStateDone):
		result = OperationStateDone
	case string(OperationStateClientError):
		result = OperationStateClientError
	case string(OperationStateError):
		result = OperationStateError
	case string(OperationStateFailed):
		result = OperationStateFailed
	case string(OperationStateOrphan):
		result = OperationStateOrphan
	default:
		return "", fmt.Errorf("operation state '%s' does not exist", state)
	}
	return result, nil
}

func (o OperationState) IsError() bool {
	return o == OperationStateError || o == OperationStateFailed || o == OperationStateClientError
}

func (o OperationState) IsFinal() bool {
	return o == OperationStateError || o == OperationStateDone
}

func (o OperationState) IsTemporary() bool {
	return !o.IsFinal()
}
