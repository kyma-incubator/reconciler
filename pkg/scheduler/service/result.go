package service

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type ReconciliationResult struct {
	logger      *zap.SugaredLogger
	reconEntity *model.ReconciliationEntity
	done        []*model.OperationEntity
	error       []*model.OperationEntity
	running     []*model.OperationEntity
	new         []*model.OperationEntity
}

func newReconciliationResult(reconEntity *model.ReconciliationEntity, logger *zap.SugaredLogger) *ReconciliationResult {
	return &ReconciliationResult{
		logger:      logger,
		reconEntity: reconEntity,
	}
}

func (rs *ReconciliationResult) Reconciliation() *model.ReconciliationEntity {
	return rs.reconEntity
}

func (rs *ReconciliationResult) AddOperations(ops []*model.OperationEntity) error {
	for _, op := range ops {
		if err := rs.AddOperation(op); err != nil {
			return errors.Wrap(err, "failed to add operations to reconciliation result")
		}
	}
	return nil
}

func (rs *ReconciliationResult) AddOperation(op *model.OperationEntity) error {
	if op.SchedulingID != rs.reconEntity.SchedulingID {
		return fmt.Errorf("cannot add operation with schedulingID '%s' "+
			"to reconciliation status with schedulingID '%s'", op.SchedulingID, rs.reconEntity.SchedulingID)
	}

	switch op.State {
	case model.OperationStateDone:
		rs.done = append(rs.done, op)
	case model.OperationStateError:
		rs.error = append(rs.error, op)
	case model.OperationStateNew:
		rs.new = append(rs.new, op)
	case model.OperationStateOrphan: //orphans will be treated like new operations
		rs.new = append(rs.new, op)
	default:
		rs.running = append(rs.running, op)
	}

	return nil
}

func (rs *ReconciliationResult) GetOperations() []*model.OperationEntity {
	var result []*model.OperationEntity
	result = append(result, rs.new...)
	result = append(result, rs.running...)
	result = append(result, rs.done...)
	return append(result, rs.error...)
}

func (rs *ReconciliationResult) GetResult() model.Status {
	isDelete := true
	for _, op := range rs.GetOperations() {
		if op.Type != model.OperationTypeDelete {
			isDelete = false
			break
		}
	}
	//this if-clause has always to be evaluated first:
	//as soon as one operation is in an error state the cluster is marked to be in error-state if no other ops are running
	if len(rs.error) > 0 && len(rs.running) == 0 {
		if isDelete {
			return model.ClusterStatusDeleteError
		}
		return model.ClusterStatusReconcileError
	}

	//this if-clause has always to be evaluated as second condition:
	//if one operation is not in a final state, the cluster is still in reconciling-state
	if len(rs.running) > 0 || len(rs.new) > 0 {
		if isDelete {
			return model.ClusterStatusDeleting
		}
		return model.ClusterStatusReconciling
	}
	//only if no operations are ongoing or in an error state, a cluster can be set to ready-state
	if len(rs.done) > 0 {
		if isDelete {
			return model.ClusterStatusDeleted
		}
		return model.ClusterStatusReady
	}
	// this should never be returned
	return model.ClusterStatusReconcileError
}

func (rs *ReconciliationResult) GetOrphans(timeout time.Duration) []*model.OperationEntity {
	var orphaned []*model.OperationEntity
	for _, op := range rs.running {
		lastUpdateAgo := time.Now().UTC().Sub(op.Updated)
		if lastUpdateAgo >= timeout {
			rs.logger.Debugf("Reconciliation result detected orphan operation '%s': "+
				"last updated is %.1f secs ago (orphan-timeout: %.1f secs)",
				op, lastUpdateAgo.Seconds(), timeout.Seconds())
			orphaned = append(orphaned, op)
		}
	}
	return orphaned
}
