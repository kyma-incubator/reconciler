package model

import (
	"fmt"
	"strings"
)

type OperationType string

const (
	OperationTypeReconcile OperationType = "reconcile"
	OperationTypeDelete    OperationType = "delete"
)

func NewOperationType(state string) (OperationType, error) {
	var result OperationType
	switch strings.ToLower(state) {
	case string(OperationTypeReconcile):
		result = OperationTypeReconcile
	case string(OperationTypeDelete):
		result = OperationTypeDelete
	default:
		return "", fmt.Errorf("operation state '%s' does not exist", state)
	}
	return result, nil
}
