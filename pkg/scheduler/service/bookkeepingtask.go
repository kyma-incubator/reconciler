package service

import (
	"errors"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"go.uber.org/zap"
	"time"
)

type BookkeepingTask interface {
	Apply(*ReconciliationResult) (error, int)
}

type orphanOperation struct{
	transition *ClusterStatusTransition
	logger *zap.SugaredLogger
}

func (oo orphanOperation) Apply(reconResult *ReconciliationResult) (error, int) {
	errMsg := ""
	errCnt := 0
	for _, orphanOp := range reconResult.GetOrphans() {
		if orphanOp.State == model.OperationStateOrphan {
			//don't update orphan operations which are already marked as 'orphan'
			continue
		}

		if err := oo.transition.reconRepo.UpdateOperationState(
			orphanOp.SchedulingID, orphanOp.CorrelationID, model.OperationStateOrphan); err == nil {
			oo.logger.Infof("orphanOperation marked operation '%s' as orphan: "+
				"last update %.2f minutes ago)", orphanOp, time.Since(orphanOp.Updated).Minutes())
		} else {
			errCnt++
			errMsg = errMsg + fmt.Sprintf("Bookkeeper failed to update status of orphan operation '%s': %s\n",
				orphanOp, err)
		}
	}
	if errMsg != "" {
		return errors.New(errMsg), errCnt
	}
	return nil, 0
}

type finishOperation struct {
	transition *ClusterStatusTransition
	logger *zap.SugaredLogger
}

func (fo finishOperation) Apply(reconResult *ReconciliationResult) (error, int) {
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
			return nil, 0
		}
		errMsg = fmt.Sprintf("finishOperation failed to update cluster '%s' to status '%s' "+
			"(triggered by reconciliation with schedulingID '%s'): %s",
			recon.RuntimeID, newClusterStatus, recon.SchedulingID, err)
		fo.logger.Errorf(errMsg)
	}

	return errors.New(errMsg), 1
}
