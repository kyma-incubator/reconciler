package service

import (
	"context"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"go.uber.org/zap"
)

type CleanerConfig struct {
	PurgeEntitiesOlderThan       time.Duration
	CleanerInterval              time.Duration
	KeepLatestEntitiesCount      *uint
	KeepUnsuccessfulEntitiesDays *uint
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
	c.logger.Infof("Starting entities cleaner: interval for clearing old Reconciliation and Operation entities "+
		"is %s. Cleaner will remove entities older than %s", config.CleanerInterval.String(), config.PurgeEntitiesOlderThan.String())

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
	if config.KeepLatestEntitiesCount != nil {
		c.purgeReconciliationsNew(transition, config)
	} else {
		c.purgeReconciliationsOld(transition, config)
	}
}

func (c *cleaner) purgeReconciliationsNew(transition *ClusterStatusTransition, config *CleanerConfig) {

	latestReconciliations, err := c.getLatestReconciliations(transition, config)
	if err != nil {
		c.logger.Errorf("Cleaner failed to get last %d reconciliations: %s", *config.KeepLatestEntitiesCount, err.Error())
	}

	if len(latestReconciliations) <= int(*config.KeepLatestEntitiesCount) {
		//Nothing to clean up
		return
	}

	oldestInRange := findOldestReconciliation(latestReconciliations)
	oldestReconciliationAgeDays := daysAgoFor(oldestInRange.Created)

	if oldestReconciliationAgeDays > int(*config.KeepUnsuccessfulEntitiesDays) {
		//The set of last 'N' reconciliations (which we must keep) contains an entity that is older than configured 'KeepUnsuccessfulEntitiesDays'
		//It's enough to drop all records older than the oldest from the set.
		err = c.dropRecordsOlderThan(oldestInRange.Created)
		if err != nil {
			c.logger.Errorf("Cleaner failed to remove reconciliations older than %s: %s", oldestInRange.Created.String(), err.Error())
		}
		return
	}

	//if we're here, there might exist unsuccessful entities older than 'oldestInRange', but within 'KeepUnsuccessfulEntitiesDays' time range.
	//We have to preserve these (if exist) and remove everything else.
	deadline := beginningOfTheDay(time.Now()).AddDate(0, 0, -1*int(*config.KeepUnsuccessfulEntitiesDays))
	err = c.dropRecordsOlderThan(deadline)
	if err != nil {
		c.logger.Errorf("Cleaner failed to remove reconciliations older than %s: %s", deadline.String(), err.Error())
	}
	err = c.dropSuccessfulRecordsOlderThan(oldestInRange.Created)
	if err != nil {
		c.logger.Errorf("Cleaner failed to remove successful reconciliations older than %s: %s", deadline.String(), err.Error())
	}
}

func (c *cleaner) purgeReconciliationsOld(transition *ClusterStatusTransition, config *CleanerConfig) {
	deadline := time.Now().UTC().Add(-1 * config.PurgeEntitiesOlderThan)
	reconciliations, err := transition.ReconciliationRepository().GetReconciliations(&reconciliation.WithCreationDateBefore{
		Time: deadline,
	})
	if err != nil {
		c.logger.Errorf("Cleaner failed to get reconciliations older than %s: %s", deadline.String(), err.Error())
	}

	for i := range reconciliations {
		c.logger.Infof("Cleaner triggered for the Reconciliation and dependent Operations with SchedulingID '%s' "+
			"(created: %s)", reconciliations[i].SchedulingID, reconciliations[i].Created)

		id := reconciliations[i].SchedulingID
		err := transition.ReconciliationRepository().RemoveReconciliation(id)
		if err != nil {
			c.logger.Errorf("Cleaner failed to remove reconciliation with schedulingID '%s': %s", id, err.Error())
		}
	}
}

func (c *cleaner) getLatestReconciliations(transition *ClusterStatusTransition, config *CleanerConfig) ([]*model.ReconciliationEntity, error) {
	//TODO: Implement
	return nil, nil
}

func (c *cleaner) dropRecordsOlderThan(t time.Time) error {
	//TODO: Implement
	return nil
}

func (c *cleaner) dropSuccessfulRecordsOlderThan(t time.Time) error {
	//TODO: Implement
	return nil
}

func findOldestReconciliation(list []*model.ReconciliationEntity) *model.ReconciliationEntity {
	//TODO: Implement
	if list != nil {
		return list[0]
	}
	return nil
}

func daysAgoFor(t time.Time) int {
	//TODO: Implement
	return 0
}

func beginningOfTheDay(t time.Time) time.Time {
	//TODO: Implement
	return time.Now()
}
