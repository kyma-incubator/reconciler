package service

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"go.uber.org/zap"
)

const (
	defaultOperationsWatchInterval = 30 * time.Second
	defaultOrphanOperationTimeout  = 5 * time.Minute
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

	ticker := time.NewTicker(bk.config.OperationsWatchInterval)
	for {
		select {
		case <-ticker.C:
			ops, err := bk.transition.ReconciliationRepository().GetReconcilingOperations()
			if err != nil {
				bk.logger.Errorf("Failed to retrieve operations of currently running reconciliations: %s", err)
				continue
			}
			filterResults, err := bk.processReconciliations(ops)
			if err != nil {
				bk.logger.Errorf("Processing of reconciliations statuses failed: %s", err)
				continue
			}

			//finish reconciliations
			for _, filterResult := range filterResults {
				reconResult := filterResult.GetResult()

				bk.logger.Debugf("Update reconcilation '%s': status is '%s'",
					filterResult.schedulingID, filterResult.GetResult())

				if err := bk.transition.FinishReconciliation(filterResult.schedulingID, reconResult); err != nil {
					bk.logger.Errorf("Bookeeper failed to update cluster status: %s", err)
				}

				//reset orphaned operations
				for _, orphanOp := range filterResult.GetOrphans() {
					bk.logger.Debugf("Marking operation '%s' (schedulingID:%s/correlationID:%s) as orphaned",
						orphanOp.SchedulingID, orphanOp.SchedulingID, orphanOp.CorrelationID)
					if err := bk.transition.ReconciliationRepository().UpdateOperationState(
						orphanOp.SchedulingID, orphanOp.CorrelationID, model.OperationStateOrphan); err != nil {
						bk.logger.Errorf("Failed to update status of orphan operation '%s' "+
							"(schedulingID:%s/correlationID:%s): %s",
							orphanOp, orphanOp.SchedulingID, orphanOp.CorrelationID, err)
					}
				}
			}
		case <-ctx.Done():
			bk.logger.Debug("Stop cluster status updater because parent context got closed")
			ticker.Stop()
			return nil
		}
	}
}

func (bk *bookkeeper) processReconciliations(ops []*model.OperationEntity) (map[string]*reconciliationStatus, error) {
	reconStatuses := make(map[string]*reconciliationStatus)
	for _, op := range ops {
		reconStatus, ok := reconStatuses[op.SchedulingID]
		if !ok {
			reconStatus = newReconciliationStatus(op.SchedulingID, bk.config.OrphanOperationTimeout)
			reconStatuses[op.SchedulingID] = reconStatus
		}
		if err := reconStatus.Add(op); err != nil {
			return nil, err
		}
	}
	return reconStatuses, nil
}

type reconciliationStatus struct {
	schedulingID  string
	orphanTimeout time.Duration
	done          []*model.OperationEntity
	error         []*model.OperationEntity
	other         []*model.OperationEntity
}

func newReconciliationStatus(schedulingID string, orphanTimeout time.Duration) *reconciliationStatus {
	return &reconciliationStatus{
		schedulingID:  schedulingID,
		orphanTimeout: orphanTimeout,
	}
}

func (rs *reconciliationStatus) Add(op *model.OperationEntity) error {
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

func (rs *reconciliationStatus) GetResult() model.Status {
	if len(rs.error) > 0 {
		return model.ClusterStatusError
	}
	if len(rs.other) > 0 {
		return model.ClusterStatusReconciling
	}
	return model.ClusterStatusReady
}

func (rs *reconciliationStatus) GetOrphans() []*model.OperationEntity {
	var orphaned []*model.OperationEntity
	for _, op := range rs.other {
		if time.Since(op.Updated) >= rs.orphanTimeout {
			orphaned = append(orphaned, op)
		}
	}
	return orphaned
}
