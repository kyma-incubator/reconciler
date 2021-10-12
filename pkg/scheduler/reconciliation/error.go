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
		cluster:      entity.RuntimeID,
		schedulingID: entity.SchedulingID,
	}
}

func IsDuplicateClusterReconciliationError(err error) bool {
	_, ok := err.(*DuplicateClusterReconciliationError)
	return ok
}
