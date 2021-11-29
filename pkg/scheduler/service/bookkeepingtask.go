package service

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

type BookkeepingTask interface {
	Apply(*ReconciliationResult) []error
}

type markOrphanOperation struct {
	transition *ClusterStatusTransition
	logger     *zap.SugaredLogger
}

func (oo markOrphanOperation) Apply(reconResult *ReconciliationResult) []error {
	var result []error = nil
	for _, orphanOp := range reconResult.GetOrphans() {
		if orphanOp.State == model.OperationStateOrphan {
			//don't update orphan operations which are already marked as 'orphan'
			continue
		}

		if err := oo.transition.reconRepo.UpdateOperationState(orphanOp.SchedulingID, orphanOp.CorrelationID, model.OperationStateOrphan, true); err == nil {
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

func (fo finishOperation) Apply(reconResult *ReconciliationResult) []error {
	recon := reconResult.Reconciliation()
	newClusterStatus := reconResult.GetResult()
	errMsg := fmt.Sprintf("finishOperation failed to update cluster '%s' to status '%s' "+
		"(triggered by reconciliation with schedulingID '%s'): CLuster is already in final state",
		recon.RuntimeID, newClusterStatus, recon.SchedulingID)

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
