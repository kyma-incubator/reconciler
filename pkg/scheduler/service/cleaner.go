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

func (c *CleanerConfig) keepLatestEntitiesCount() int {
	if c.KeepLatestEntitiesCount == nil {
		panic("Can't convert KeepLatestEntitiesCount to int: is nil")
	}

	return toInt(c.KeepLatestEntitiesCount)
}

func (c *CleanerConfig) keepUnsuccessfulEntitiesDays() int {
	if c.KeepUnsuccessfulEntitiesDays == nil {
		panic("Can't convert KeepUnsuccessfulEntitiesDays to int: is nil")
	}

	return toInt(c.KeepUnsuccessfulEntitiesDays)
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

//Purges reconciliations using rules from: https://github.com/kyma-incubator/reconciler/issues/668
func (c *cleaner) purgeReconciliationsNew(transition *ClusterStatusTransition, config *CleanerConfig) {

	latestReconciliations, err := c.getLatestReconciliations(transition, config.keepLatestEntitiesCount())
	if err != nil {
		c.logger.Errorf("Cleaner failed to get last %d reconciliations: %s", config.keepLatestEntitiesCount(), err.Error())
	}

	if len(latestReconciliations) <= config.keepLatestEntitiesCount() {
		//Nothing to clean up
		return
	}

	oldestInRange := findOldestReconciliation(latestReconciliations)
	oldestReconciliationAgeDays := diffDays(oldestInRange.Created, time.Now())

	if oldestReconciliationAgeDays > config.keepUnsuccessfulEntitiesDays() {
		//The set of last 'N' reconciliations (which we must keep) contains an entity that is older than configured 'KeepUnsuccessfulEntitiesDays'
		//It's enough to drop all records older than the oldest from the set.
		err = c.dropRecordsOlderThan(transition, oldestInRange.Created)
		if err != nil {
			c.logger.Errorf("Cleaner failed to remove all reconciliations older than %s: %s", oldestInRange.Created.String(), err.Error())
		}
		return
	}

	//if we're here, there may exist unsuccessful entities older than 'oldestInRange', but within 'KeepUnsuccessfulEntitiesDays' time range.
	//We have to preserve these (if exist) and remove everything else.
	deadline := beginningOfTheDay(time.Now()).AddDate(0, 0, -1*config.keepUnsuccessfulEntitiesDays())
	err = c.dropRecordsOlderThan(transition, deadline)
	if err != nil {
		c.logger.Errorf("Cleaner failed to remove reconciliations older than %s: %s", deadline.String(), err.Error())
	}
	err = c.dropSuccessfulRecordsOlderThan(transition, oldestInRange.Created)
	if err != nil {
		c.logger.Errorf("Cleaner failed to remove all successful reconciliations older than %s: %s", deadline.String(), err.Error())
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

func (c *cleaner) getLatestReconciliations(transition *ClusterStatusTransition, keepLatestEntitiesCount int) ([]*model.ReconciliationEntity, error) {
	return transition.ReconciliationRepository().GetReconciliations(&reconciliation.Limit{Count: keepLatestEntitiesCount})
}

func (c *cleaner) dropRecordsOlderThan(transition *ClusterStatusTransition, t time.Time) error {
	deadline := t.UTC()
	reconciliations, err := transition.ReconciliationRepository().GetReconciliations(&reconciliation.WithCreationDateBefore{
		Time: deadline,
	})

	if err != nil {
		c.logger.Errorf("Cleaner failed to get reconciliations older than %s: %s", deadline.String(), err.Error())
		return err
	}

	for i := range reconciliations {
		id := reconciliations[i].SchedulingID
		err := transition.ReconciliationRepository().RemoveReconciliation(id)
		if err != nil {
			c.logger.Errorf("Cleaner failed to remove reconciliation with schedulingID '%s': %s", id, err.Error())
		}
	}

	return err
}

func (c *cleaner) dropSuccessfulRecordsOlderThan(transition *ClusterStatusTransition, t time.Time) error {
	deadline := t.UTC()
	reconciliations, err := transition.ReconciliationRepository().GetReconciliations(&reconciliation.WithCreationDateBefore{
		Time: deadline,
	})
	if err != nil {
		c.logger.Errorf("Cleaner failed to get reconciliations older than %s: %s", deadline.String(), err.Error())
		return err
	}

	for i := range reconciliations {
		id := reconciliations[i].SchedulingID
		//TODO: does this mean "successful" ?
		if reconciliations[i].Status.IsFinalStable() {
			err := transition.ReconciliationRepository().RemoveReconciliation(id)
			if err != nil {
				c.logger.Errorf("Cleaner failed to remove reconciliation with schedulingID '%s': %s", id, err.Error())
			}
		}
	}

	return err
}

func findOldestReconciliation(list []*model.ReconciliationEntity) *model.ReconciliationEntity {
	if len(list) == 0 {
		return nil
	}

	oldest := list[0]

	for i := 1; i < len(list); i++ {
		if list[i].Created.Before(oldest.Created) {
			oldest = list[i]
		}
	}

	return oldest
}

func diffDays(earlier, later time.Time) int {
	t1 := earlier.UTC()
	t2 := later.UTC()

	if !t1.Before(t2) {
		return 0
	}

	diff := t2.Sub(t1).Hours()
	return int(diff / 24)
}

//beginningOfTheDay returns t truncated to the very beginning of the day
func beginningOfTheDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func toInt(up *uint) int {
	return int(*up)
}
