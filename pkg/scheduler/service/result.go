package service

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type ReconciliationResult struct {
	logger        *zap.SugaredLogger
	reconEntity   *model.ReconciliationEntity
	orphanTimeout time.Duration
	done          []*model.OperationEntity
	error         []*model.OperationEntity
	other         []*model.OperationEntity
}

func newReconciliationResult(reconEntity *model.ReconciliationEntity, orphanTimeout time.Duration, logger *zap.SugaredLogger) *ReconciliationResult {
	return &ReconciliationResult{
		logger:        logger,
		reconEntity:   reconEntity,
		orphanTimeout: orphanTimeout,
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
	default:
		rs.other = append(rs.other, op)
	}

	return nil
}

func (rs *ReconciliationResult) GetOperations() []*model.OperationEntity {
	var result []*model.OperationEntity
	result = append(result, rs.other...)
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
	if len(rs.error) > 0 {
		if isDelete {
			return model.ClusterStatusDeleteError
		}
		return model.ClusterStatusReconcileError
	}
	if len(rs.other) > 0 {
		if isDelete {
			return model.ClusterStatusDeleting
		}
		return model.ClusterStatusReconciling
	}
	if len(rs.done) > 0 {
		if isDelete {
			return model.ClusterStatusDeleted
		}
		return model.ClusterStatusReady
	}
	// this should never be returned
	return model.ClusterStatusReconcileError
}

func (rs *ReconciliationResult) GetOrphans() []*model.OperationEntity {
	var orphaned []*model.OperationEntity
	for _, op := range rs.other {
		lastUpdateAgo := time.Now().UTC().Sub(op.Updated)
		if lastUpdateAgo >= rs.orphanTimeout {
			rs.logger.Debugf("Reconciliation result detected orphan operation '%s': "+
				"last updated is %.1f secs ago (orphan-timeout: %.1f secs)",
				op, lastUpdateAgo.Seconds(), rs.orphanTimeout.Seconds())
			orphaned = append(orphaned, op)
		}
	}
	return orphaned
}
