package reconciliation

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
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

type EmptyComponentsReconciliationError struct {
	state *cluster.State
}

func (err *EmptyComponentsReconciliationError) Error() string {
	return fmt.Sprintf("Error creating reconciliation for cluster with RuntimeID: %s and ConfigID: %d, component list is empty.", err.state.Cluster.RuntimeID, err.state.Configuration.Version)
}

func newEmptyComponentsReconciliationError(state *cluster.State) error {
	return &EmptyComponentsReconciliationError{
		state: state,
	}
}

func IsEmptyComponentsReconciliationError(err error) bool {
	_, ok := err.(*EmptyComponentsReconciliationError)
	return ok
}
