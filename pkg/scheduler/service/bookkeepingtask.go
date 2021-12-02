package service

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

type BookkeepingTask interface {
	Apply(reconResult *ReconciliationResult, maxRetries int) []error
}

type markOrphanOperation struct {
	transition *ClusterStatusTransition
	logger     *zap.SugaredLogger
}

func (oo markOrphanOperation) Apply(reconResult *ReconciliationResult, maxRetries int) []error {
	var result []error = nil
	for _, orphanOp := range reconResult.GetOrphans() {
		if orphanOp.State == model.OperationStateOrphan {
			//don't update orphan operations which are already marked as 'orphan'
			continue
		}

		if err := oo.transition.reconRepo.UpdateOperationState(orphanOp.SchedulingID, orphanOp.CorrelationID, model.OperationStateOrphan, false); err == nil {
			oo.logger.Infof("markOrphanOperation marked operation '%s' as orphan: "+
				"last update %.2f minutes ago)", orphanOp, time.Since(orphanOp.Updated).Minutes())
		} else {
			result = append(result, errors.Wrap(err, fmt.Sprintf("Bookkeeper failed to update status of orphan operation %s", orphanOp)))
		}
	}
	return result
}

type finishOperation struct {
	transition *ClusterStatusTransition
	logger     *zap.SugaredLogger
}

func (fo finishOperation) Apply(reconResult *ReconciliationResult, maxRetries int) []error {
	recon := reconResult.Reconciliation()
	newClusterStatus := reconResult.GetResult()
	errMsg := fmt.Sprintf("finishOperation failed to update cluster '%s' to status '%s' "+
		"(triggered by reconciliation with schedulingID '%s'): CLuster is already in final state",
		recon.RuntimeID, newClusterStatus, recon.SchedulingID)

	if newClusterStatus == model.ClusterStatusReconcileError {
		errCnt, err := fo.transition.inventory.CountRetries(reconResult.reconEntity.RuntimeID, reconResult.reconEntity.ClusterConfig)
		if err != nil {
			fo.logger.Errorf("failed to count error for runtime %s with error: %s", reconResult.reconEntity.RuntimeID, err)
		}
		if errCnt < maxRetries {
			newClusterStatus = model.ClusterStatusReconcileErrorRetryable
			fo.logger.Infof("Reconciliation for cluster with runtimeID '%s' and clusterConfig '%d' failed but "+
				"reconciliation will be retried (count of applied retries: %d)",
				reconResult.reconEntity.RuntimeID, reconResult.reconEntity.ClusterConfig, errCnt)
		}
	}

	if newClusterStatus.IsFinal() {
		err := fo.transition.FinishReconciliation(recon.SchedulingID, newClusterStatus)
		if err == nil {
			fo.logger.Infof("finishOperation updated cluster '%s' to status '%s' "+
				"(triggered by reconciliation with schedulingID '%s')",
				recon.RuntimeID, newClusterStatus, recon.SchedulingID)
			return nil
		}
		errMsg = fmt.Sprintf("finishOperation failed to update cluster '%s' to status '%s' "+
			"(triggered by reconciliation with schedulingID '%s'): %s",
			recon.RuntimeID, newClusterStatus, recon.SchedulingID, err)
		fo.logger.Errorf(errMsg)
	}
	return []error{errors.New(errMsg)}
}
