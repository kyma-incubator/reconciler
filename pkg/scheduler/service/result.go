package service

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

type ReconciliationResult struct {
	logger        *zap.SugaredLogger
	schedulingID  string
	orphanTimeout time.Duration
	done          []*model.OperationEntity
	error         []*model.OperationEntity
	other         []*model.OperationEntity
}

func newReconciliationResult(schedulingID string, orphanTimeout time.Duration, logger *zap.SugaredLogger) *ReconciliationResult {
	return &ReconciliationResult{
		logger:        logger,
		schedulingID:  schedulingID,
		orphanTimeout: orphanTimeout,
	}
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
	if op.SchedulingID != rs.schedulingID {
		return fmt.Errorf("cannot add operation with schedulingID '%s' "+
			"to reconciliation status with schedulingID '%s'", op.SchedulingID, rs.schedulingID)
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
	if len(rs.error) > 0 {
		return model.ClusterStatusError
	}
	if len(rs.other) > 0 {
		return model.ClusterStatusReconciling
	}
	return model.ClusterStatusReady
}

func (rs *ReconciliationResult) GetOrphans() []*model.OperationEntity {
	var orphaned []*model.OperationEntity
	for _, op := range rs.other {
		lastUpdateAgo := time.Now().UTC().Sub(op.Updated)
		opIsOrphan := lastUpdateAgo >= rs.orphanTimeout
		rs.logger.Debugf("Reconciliation result verified operation '%s': "+
			"last updated is %f.1f secs ago (orphan-timeout: %.1f secs / op is orphan: %t)",
			op, lastUpdateAgo.Seconds(), rs.orphanTimeout.Seconds(), opIsOrphan)
		if opIsOrphan {
			orphaned = append(orphaned, op)
		}
	}
	return orphaned
}
