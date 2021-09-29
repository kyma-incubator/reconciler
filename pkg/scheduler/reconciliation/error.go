package reconciliation

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type DuplicateClusterReconciliationError struct {
	cluster      string
	schedulingID string
}

func (err *DuplicateClusterReconciliationError) Error() string {
	return fmt.Sprintf("cluster '%s' is already considered for reconciliation (schedulingID:%s)",
		err.cluster, err.schedulingID)
}

func newDuplicateClusterReconciliationError(entity *model.ReconciliationEntity) error {
	return &DuplicateClusterReconciliationError{
		cluster:      entity.Cluster,
		schedulingID: entity.SchedulingID,
	}
}

func IsDuplicateClusterReconciliationError(err error) bool {
	_, ok := err.(*DuplicateClusterReconciliationError)
	return ok
}

type RedundantOperationStateUpdateError struct {
	op *model.OperationEntity
}

func (err *RedundantOperationStateUpdateError) Error() string {
	return fmt.Sprintf("operation '%s' is already in state '%s'",
		err.op, err.op.State)
}

func newRedundantOperationStateUpdateError(op *model.OperationEntity) error {
	return &RedundantOperationStateUpdateError{
		op: op,
	}
}

func IsRedundantOperationStateUpdateError(err error) bool {
	_, ok := err.(*RedundantOperationStateUpdateError)
	return ok
}
