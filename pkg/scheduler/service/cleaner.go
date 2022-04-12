package service

import (
	"context"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"go.uber.org/zap"
)

var CleanerPrefix = "[CLEANER]"

type CleanerConfig struct {
	PurgeEntitiesOlderThan  time.Duration
	CleanerInterval         time.Duration
	KeepLatestEntitiesCount uint
	MaxEntitiesAgeDays      uint
	StatusCleanupBatchSize  uint
}

func (c *CleanerConfig) keepLatestEntitiesCount() int {
	return int(c.KeepLatestEntitiesCount)
}

func (c *CleanerConfig) maxEntitiesAgeDays() int {
	return int(c.MaxEntitiesAgeDays)
}

func (c *CleanerConfig) statusCleanupBatchSize() int {
	return int(c.StatusCleanupBatchSize)
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
	c.logger.Infof("%s Starting entities cleaner: interval for clearing old cluster inventory and reconciliation entities is %s", CleanerPrefix, config.CleanerInterval.String())

	ticker := time.NewTicker(config.CleanerInterval)
	c.purgeEntities(transition, config) //check for entities now, otherwise first check would be trigger by ticker
	for {
		select {
		case <-ticker.C:
			c.purgeEntities(transition, config)
		case <-ctx.Done():
			c.logger.Infof("%s Stopping because parent context got closed", CleanerPrefix)
			ticker.Stop()
			return nil
		}
	}
}

func (c *cleaner) purgeEntities(transition *ClusterStatusTransition, config *CleanerConfig) {
	c.logger.Infof("%s Process started", CleanerPrefix)

	// delete reconciliations
	if config.keepLatestEntitiesCount() > 0 {
		c.logger.Infof("%s Cleaner will remove unnecessary reconciliations", CleanerPrefix)
		c.purgeReconciliationsNew(transition, config)
	} else {
		c.logger.Infof("%s Cleaner will remove reconciliations older than %s", CleanerPrefix, config.PurgeEntitiesOlderThan.String())
		c.purgeReconciliationsOld(transition, config)
	}

	// delete cluster entities
	clusterInventoryCleanupDays := 20 // days default
	if config.maxEntitiesAgeDays() > 0 {
		clusterInventoryCleanupDays = config.maxEntitiesAgeDays()
	}
	c.logger.Infof("%s Cleaner will remove inventory clusters and intermediary statuses", CleanerPrefix)
	deadline := beginningOfTheDay(time.Now().UTC()).AddDate(0, 0, -1*clusterInventoryCleanupDays)
	if err := transition.CleanStatusesAndDeletedClustersOlderThan(deadline, config.statusCleanupBatchSize()); err != nil {
		c.logger.Errorf("%s Failed to remove inventory clusters and intermediary statuses %v", CleanerPrefix, err)
	}

	c.logger.Infof("%s Process finished", CleanerPrefix)
}

//Purges reconciliations using rules from: https://github.com/kyma-incubator/reconciler/issues/668
func (c *cleaner) purgeReconciliationsNew(transition *ClusterStatusTransition, config *CleanerConfig) {

	runtimeIDs, err := transition.ReconciliationRepository().GetRuntimeIDs()
	if err != nil {
		c.logger.Errorf("%s Failed to get all runtimeIDs: %s", CleanerPrefix, err.Error())
		return
	}

	for _, runtimeID := range runtimeIDs {
		c.purgeReconciliationsForCluster(runtimeID, transition, config)
	}
}

func (c *cleaner) purgeReconciliationsForCluster(runtimeID string, transition *ClusterStatusTransition, config *CleanerConfig) {
	c.logger.Infof("%s Cleaning reconciliation entries for cluster with RuntimeID: %s", CleanerPrefix, runtimeID)

	//1. Bulk delete old records, keeping the most recent one (should never be deleted)
	if err := c.deleteRecordsByAge(runtimeID, config.maxEntitiesAgeDays(), transition); err != nil {
		c.logger.Errorf("%s Failed to delete reconciliations older than %d days: %s", CleanerPrefix, config.maxEntitiesAgeDays(), err.Error())
		return
	}

	//2. Delete remaining records according to "count" and "status" criteria.
	if err := c.deleteRecordsByCountAndStatus(runtimeID, transition, config); err != nil {
		c.logger.Errorf("%s Failed to delete reconciliations more than %d: %s", CleanerPrefix, config.keepLatestEntitiesCount(), err.Error())
		return
	}
	c.logger.Infof("%s Done cleaning reconciliation entries for cluster with RuntimeID: %s", CleanerPrefix, runtimeID)
}

//deleteRecordsByAge deletes all reconciliations for a given cluster that's older than configured number of days except the single most recent record - that one is never deleted
func (c *cleaner) deleteRecordsByAge(runtimeID string, numberOfDays int, transition *ClusterStatusTransition) error {
	now := time.Now()
	deadline := beginningOfTheDay(now.UTC()).AddDate(0, 0, -1*numberOfDays)

	c.logger.Infof("%s Removing reconciliations older than: %s except the most recent one for the cluster %s", CleanerPrefix, deadline.UTC().String(), runtimeID)

	mostRecentReconciliation, err := c.getMostRecentReconciliation(runtimeID, transition)
	if err != nil {
		return err
	}
	if mostRecentReconciliation == nil {
		c.logger.Infof("%s No reconciliations found for the cluster: %s", CleanerPrefix, runtimeID)
		return nil
	}

	return transition.ReconciliationRepository().RemoveReconciliationsBeforeDeadline(runtimeID, mostRecentReconciliation.SchedulingID, deadline)
}

//deleteRecordsByCountAndStatus deletes record between some deadline in the past and now. It keeps the config.KeepLatestEntitiesCount() of the most recent records and the ones that are not successfully finished.
func (c *cleaner) deleteRecordsByCountAndStatus(runtimeID string, transition *ClusterStatusTransition, config *CleanerConfig) error {
	//Note: This functions assumes that deleteRecordsByAge() has already deleted records older than the "deadline"!

	mostRecentEntitiesToKeep := config.keepLatestEntitiesCount()
	if mostRecentEntitiesToKeep == 0 {
		return nil
	}

	reconciliations, err := transition.ReconciliationRepository().GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: runtimeID})
	if err != nil {
		return err
	}

	if len(reconciliations) <= mostRecentEntitiesToKeep {
		return nil
	}

	var schedulingIDsToDrop []interface{}
	for _, obsoleteReconciliation := range reconciliations[mostRecentEntitiesToKeep:] {
		if obsoleteReconciliation.Status.IsFinalStable() {
			schedulingIDsToDrop = append(schedulingIDsToDrop, obsoleteReconciliation.SchedulingID)
		}
	}

	if len(schedulingIDsToDrop) > 0 {
		c.logger.Debugf("%s Found %d records with a \"successful\" status to delete for the cluster %s", CleanerPrefix, len(schedulingIDsToDrop), runtimeID)
		return c.removeReconciliations(schedulingIDsToDrop, transition)
	}

	return nil
}

func (c *cleaner) getMostRecentReconciliation(runtimeID string, transition *ClusterStatusTransition) (*model.ReconciliationEntity, error) {
	limitFilter := reconciliation.Limit{Count: 1}
	runtimeIDFilter := reconciliation.WithRuntimeID{RuntimeID: runtimeID}

	filter := reconciliation.FilterMixer{Filters: []reconciliation.Filter{&runtimeIDFilter, &limitFilter}}

	res, err := transition.ReconciliationRepository().GetReconciliations(&filter)
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, nil
	}

	return res[0], nil
}

//removeReconciliations drops all reconciliations provided in the list
func (c *cleaner) removeReconciliations(schedulingIDs []interface{}, transition *ClusterStatusTransition) error {
	if err := transition.ReconciliationRepository().RemoveReconciliationsBySchedulingID(schedulingIDs); err != nil {
		c.logger.Errorf("%s Failed to remove reconciliations: %v", CleanerPrefix, err.Error())
		return err
	}
	c.logger.Infof("%s Removed %d reconciliation (finished)", CleanerPrefix, len(schedulingIDs))
	return nil
}

func (c *cleaner) purgeReconciliationsOld(transition *ClusterStatusTransition, config *CleanerConfig) {
	deadline := time.Now().UTC().Add(-1 * config.PurgeEntitiesOlderThan)
	reconciliations, err := transition.ReconciliationRepository().GetReconciliations(&reconciliation.WithCreationDateBefore{
		Time: deadline,
	})
	if err != nil {
		c.logger.Errorf("%s Failed to get reconciliations older than %s: %s", CleanerPrefix, deadline.String(), err.Error())
	}

	for i := range reconciliations {
		id := reconciliations[i].SchedulingID
		err := transition.ReconciliationRepository().RemoveReconciliationBySchedulingID(id)
		if err != nil {
			c.logger.Errorf("%s Failed to remove reconciliation with schedulingID '%s': %s", CleanerPrefix, id, err.Error())
		}
	}
}

//beginningOfTheDay returns t truncated to the very beginning of the day
func beginningOfTheDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
