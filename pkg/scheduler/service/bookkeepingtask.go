package service

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

type BookkeepingTask interface {
	Apply(reconResult *ReconciliationResult, config *BookkeeperConfig) []error
}

type markOrphanOperation struct {
	transition *ClusterStatusTransition
	logger     *zap.SugaredLogger
}

func (oo markOrphanOperation) Apply(reconResult *ReconciliationResult, config *BookkeeperConfig) []error {
	var result []error = nil
	for _, orphanOp := range reconResult.GetOrphans(config.OrphanOperationTimeout) {
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

func (fo finishOperation) Apply(reconResult *ReconciliationResult, config *BookkeeperConfig) []error {
	recon := reconResult.Reconciliation()
	newClusterStatus := reconResult.GetResult()

	if newClusterStatus == model.ClusterStatusReconcileError {
		errCnt, err := fo.transition.inventory.CountRetries(reconResult.reconEntity.RuntimeID, reconResult.reconEntity.ClusterConfig, config.MaxRetries)
		if err != nil {
			fo.logger.Errorf("failed to count error for runtime %s with error: %s", reconResult.reconEntity.RuntimeID, err)
		}
		if errCnt < config.MaxRetries {
			newClusterStatus = model.ClusterStatusReconcileErrorRetryable
			fo.logger.Infof("Reconciliation for cluster with runtimeID '%s' and clusterConfig '%d' failed but "+
				"reconciliation will be retried (count of applied retries: %d)",
				reconResult.reconEntity.RuntimeID, reconResult.reconEntity.ClusterConfig, errCnt)
		}
	}

	if !newClusterStatus.IsFinal() {
		return nil
	}

	err := fo.transition.FinishReconciliation(recon.SchedulingID, newClusterStatus)
	if err == nil {
		fo.logger.Infof("finishOperation updated cluster '%s' to status '%s' "+
			"(triggered by reconciliation with schedulingID '%s')",
			recon.RuntimeID, newClusterStatus, recon.SchedulingID)
		return nil
	}

	return []error{errors.New(fmt.Sprintf("finishOperation failed to update cluster '%s' to status '%s' "+
		"(triggered by reconciliation with schedulingID '%s'): %s",
		recon.RuntimeID, newClusterStatus, recon.SchedulingID, err))}
}
