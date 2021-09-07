package model

type OperationState string

const (
	OperationStateNew         = "new"
	OperationStateInProgress  = "in_progress"
	OperationStateDone        = "done"
	OperationStateClientError = "client_error"
	OperationStateError       = "error"
	OperationStateFailed      = "failed"
)
