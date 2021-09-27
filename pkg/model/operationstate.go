package model

import (
	"fmt"
	"strings"
)

type OperationState string

const (
	OperationStateNew         = "new"
	OperationStateInProgress  = "in_progress"
	OperationStateDone        = "done"
	OperationStateClientError = "client_error"
	OperationStateError       = "error"
	OperationStateFailed      = "failed"
	OperationStateOrphan      = "orphan"
)

func NewOperationState(state string) (OperationState, error) {
	var result OperationState
	switch strings.ToLower(state) {
	case OperationStateNew:
		result = OperationStateNew
	case OperationStateInProgress:
		result = OperationStateInProgress
	case OperationStateDone:
		result = OperationStateDone
	case OperationStateClientError:
		result = OperationStateClientError
	case OperationStateError:
		result = OperationStateError
	case OperationStateFailed:
		result = OperationStateFailed
	case OperationStateOrphan:
		result = OperationStateOrphan
	default:
		return "", fmt.Errorf("operation state '%s' does not exist", state)
	}
	return result, nil
}
