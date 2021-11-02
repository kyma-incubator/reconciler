package service

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/pkg/errors"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"go.uber.org/zap"
)

const (
	defaultOperationsWatchInterval = 30 * time.Second
	defaultOrphanOperationTimeout  = 10 * time.Minute
)

type BookkeeperConfig struct {
	OperationsWatchInterval time.Duration
	OrphanOperationTimeout  time.Duration
}

func (wc *BookkeeperConfig) validate() error {
	if wc.OperationsWatchInterval < 0 {
		return errors.New("operations watch interval cannot be < 0")
	}
	if wc.OperationsWatchInterval == 0 {
		wc.OperationsWatchInterval = defaultOperationsWatchInterval
	}
	if wc.OrphanOperationTimeout < 0 {
		return errors.New("orphan operation timeout cannot be < 0")
	}
	if wc.OrphanOperationTimeout == 0 {
		wc.OrphanOperationTimeout = defaultOrphanOperationTimeout
	}
	return nil
}

type bookkeeper struct {
	transition *ClusterStatusTransition
	config     *BookkeeperConfig
	logger     *zap.SugaredLogger
}

func newBookkeeper(transition *ClusterStatusTransition, config *BookkeeperConfig, logger *zap.SugaredLogger) *bookkeeper {
	if config == nil {
		config = &BookkeeperConfig{}
	}
	return &bookkeeper{
		transition: transition,
		config:     config,
		logger:     logger,
	}
}

func (bk *bookkeeper) Run(ctx context.Context) error {
	if err := bk.config.validate(); err != nil {
		return err
	}

	bk.logger.Infof("Starting bookkeeper: interval for updating reconciliation statuses and orphan operations "+
		"is %.1f secs / timeout for orphan operations is %.1f secs",
		bk.config.OperationsWatchInterval.Seconds(), bk.config.OrphanOperationTimeout.Seconds())

	//IMPORTANT:
	//Bookkeeper is not allowed to run directly when Run-fct is called: is has to wait until the first ticker was fired!
	//This is important to give running component-reconciler the chance to send their heartbeat messages to mothership-
	//reconciler in case of a mothership-reconciler downtime. If bookkeeper runs directly, it would mark all ongoing
	//operations as orphan if mothership-reconciler was down for a few minutes.

	ticker := time.NewTicker(bk.config.OperationsWatchInterval)
	for {
		select {
		case <-ticker.C:
			recons, err := bk.transition.ReconciliationRepository().GetReconciliations(&reconciliation.CurrentlyReconciling{})
			if err != nil {
				bk.logger.Errorf("Bookkeeper failed to retrieve currently running reconciliations: %s", err)
				continue
			}

			for _, recon := range recons {
				reconResult, err := bk.newReconciliationResult(recon)
				if err == nil {
					bk.logger.Debugf("Bookkeeper evaluated reconciliation (schedulingID:%s) for cluster '%s' "+
						"to cluster status '%s': Done=%s / Error=%s / Other=%s",
						recon.SchedulingID, recon.RuntimeID, reconResult.GetResult(),
						bk.componentList(reconResult.done, false),
						bk.componentList(reconResult.error, true),
						bk.componentList(reconResult.other, true))
				} else {
					bk.logger.Errorf("Bookkeeper failed to retrieve operations for reconciliation '%s' "+
						"(but will continue processing): %s", recon, err)
					continue
				}
				if !bk.finishReconciliation(reconResult) {
					//check for orphan operations only if reconciliation isn't finished
					bk.markOrphanOperations(reconResult)
				}

			}
		case <-ctx.Done():
			bk.logger.Info("Stopping bookkeeper because parent context got closed")
			ticker.Stop()
			return nil
		}
	}
}

func (bk *bookkeeper) markOrphanOperations(reconResult *ReconciliationResult) {
	for _, orphanOp := range reconResult.GetOrphans() {
		if orphanOp.State == model.OperationStateOrphan {
			//don't update orphan operations which are already marked as 'orphan'
			continue
		}

		if err := bk.transition.ReconciliationRepository().UpdateOperationState(
			orphanOp.SchedulingID, orphanOp.CorrelationID, model.OperationStateOrphan); err == nil {
			bk.logger.Infof("Bookkeeper marked operation '%s' as orphan: "+
				"last update %.2f minutes ago)", orphanOp, time.Since(orphanOp.Updated).Minutes())
		} else {
			bk.logger.Errorf("Bookkeeper failed to update status of orphan operation '%s': %s",
				orphanOp, err)
		}
	}
}

func (bk *bookkeeper) finishReconciliation(reconResult *ReconciliationResult) bool {
	recon := reconResult.Reconciliation()
	newClusterStatus := reconResult.GetResult()

	if newClusterStatus.IsFinal() {
		err := bk.transition.FinishReconciliation(recon.SchedulingID, newClusterStatus)
		if err == nil {
			bk.logger.Infof("Bookkeeper updated cluster '%s' to status '%s' "+
				"(triggered by reconciliation with schedulingID '%s')",
				recon.RuntimeID, newClusterStatus, recon.SchedulingID)
			return true
		}
		bk.logger.Errorf("Bookkeeper failed to update cluster '%s' to status '%s' "+
			"(triggered by reconciliation with schedulingID '%s'): %s",
			recon.RuntimeID, newClusterStatus, recon.SchedulingID, err)
	}

	return false
}

func (bk *bookkeeper) newReconciliationResult(recon *model.ReconciliationEntity) (*ReconciliationResult, error) {
	ops, err := bk.transition.ReconciliationRepository().GetOperations(recon.SchedulingID)
	if err != nil {
		return nil, err
	}
	reconResult := newReconciliationResult(recon, bk.config.OrphanOperationTimeout, bk.logger)
	if err := reconResult.AddOperations(ops); err != nil {
		return nil, err
	}
	return reconResult, nil
}

func (bk *bookkeeper) componentList(ops []*model.OperationEntity, withReason bool) string {
	var buffer bytes.Buffer
	for _, op := range ops {
		if buffer.Len() > 0 {
			buffer.WriteRune(',')
		}
		buffer.WriteString(op.Component)
		if withReason && bk.operationHasFailureState(op) {
			buffer.WriteString(fmt.Sprintf("[error: %s]", op.Reason))
		}
	}
	return buffer.String()
}

func (bk *bookkeeper) operationHasFailureState(op *model.OperationEntity) bool {
	return op.State == model.OperationStateError ||
		op.State == model.OperationStateFailed ||
		op.State == model.OperationStateClientError
}
