package converters

import (
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/pkg/errors"
)

func ConvertReconciliation(reconciliation *model.ReconciliationEntity, operations []*model.OperationEntity) (keb.ReconciliationInfoOKResponse, error) {
	resultStatus, err := keb.ToStatus(string(reconciliation.Status))
	if err != nil {
		return keb.ReconciliationInfoOKResponse{}, errors.Wrap(err, "while converting status")
	}

	operationLen := len(operations)
	resultOperations := make([]keb.Operation, operationLen)
	for i, operation := range operations {
		resultOperations[i] = ConvertOperation(operation)
	}

	result := keb.ReconciliationInfoOKResponse{
		Created:       reconciliation.Created,
		Finished:      reconciliation.Finished,
		RuntimeID:     reconciliation.RuntimeID,
		SchedulingID:  reconciliation.SchedulingID,
		Updated:       reconciliation.Updated,
		ConfigVersion: reconciliation.ClusterConfig,
		Status:        resultStatus,
		Operations:    resultOperations,
	}

	return result, nil
}

func ConvertOperation(operation *model.OperationEntity) keb.Operation {
	if operation == nil {
		return keb.Operation{}
	}
	return keb.Operation{
		Component:     operation.Component,
		CorrelationID: operation.CorrelationID,
		Created:       operation.Created,
		Priority:      operation.Priority,
		Reason:        operation.Reason,
		SchedulingID:  operation.SchedulingID,
		State:         string(operation.State),
		Updated:       operation.Updated,
		Type:          string(operation.Type),
	}
}
