package service

import (
	"context"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"go.uber.org/zap"
)

type CleanerConfig struct {
	PurgeEntitiesOlderThan time.Duration
	CleanerInterval        time.Duration
}

type cleaner struct {
	logger *zap.SugaredLogger
}

func newCleaner(logger *zap.SugaredLogger) *cleaner {
	return &cleaner{
		logger: logger,
	}
}

func (c *cleaner) Run(ctx context.Context, transition *ClusterStatusTransition, config *CleanerConfig) error {
	c.logger.Infof("Starting entities cleaner: interval for clearing old reconciliation and operation entities "+
		"is %.1f minutes", config.PurgeEntitiesOlderThan.Minutes())

	ticker := time.NewTicker(config.CleanerInterval)
	c.purgeReconciliations(transition, config) //check for entities now, otherwise first check would be trigger by ticker
	for {
		select {
		case <-ticker.C:
			c.purgeReconciliations(transition, config)
		case <-ctx.Done():
			c.logger.Info("Stopping cleaner because parent context got closed")
			ticker.Stop()
			return nil
		}
	}
}

func (c *cleaner) purgeReconciliations(transition *ClusterStatusTransition, config *CleanerConfig) {
	cretedBefore := time.Now().Add(-1 * config.PurgeEntitiesOlderThan)
	reconciliations, err := transition.ReconciliationRepository().GetReconciliations(&reconciliation.WithCreationDateBefore{
		Time: cretedBefore,
	})
	if err != nil {
		c.logger.Error("Cleaner failed to get reconciliations older than: %s", cretedBefore)
	}

	for i := range reconciliations {
		id := reconciliations[i].SchedulingID
		err := transition.ReconciliationRepository().RemoveReconciliation(id)
		if err != nil {
			c.logger.Errorf("Cleaner failed to remove reconciliation with schedulingID: '%s'", id)
		}
	}
}
