package service

import (
	"context"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"go.uber.org/zap"
)

type CleanerConfig struct {
	PurgeEntitiesOlderThan  time.Duration
	CleanerInterval         time.Duration
	KeepLatestEntitiesCount uint
	MaxEntitiesAgeDays      uint
}

func (c *CleanerConfig) keepLatestEntitiesCount() int {
	return int(c.KeepLatestEntitiesCount)
}

func (c *CleanerConfig) maxEntitiesAgeDays() int {
	return int(c.MaxEntitiesAgeDays)
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
	c.logger.Infof("[CLEANER] Starting entities cleaner: interval for clearing old Reconciliation and Operation entities "+
		"is %s", config.CleanerInterval.String())

	ticker := time.NewTicker(config.CleanerInterval)
	c.purgeReconciliations(transition, config) //check for entities now, otherwise first check would be trigger by ticker
	for {
		select {
		case <-ticker.C:
			c.purgeReconciliations(transition, config)
		case <-ctx.Done():
			c.logger.Info("[CLEANER] Stopping because parent context got closed")
			ticker.Stop()
			return nil
		}
	}
}

func (c *cleaner) purgeReconciliations(transition *ClusterStatusTransition, config *CleanerConfig) {

	c.logger.Info("[CLEANER] Process started")

	if config.KeepLatestEntitiesCount > 0 {
		c.logger.Infof("[CLEANER] Cleaner will remove unnecessary entities (issue 884)")
		c.purgeReconciliationsNew(transition, config)
	} else {
		c.logger.Infof("[CLEANER] Cleaner will remove entities older than %s", config.PurgeEntitiesOlderThan.String())
		c.purgeReconciliationsOld(transition, config)
	}

	c.logger.Info("[CLEANER] Process finished")
}

//Purges reconciliations using rules from: https://github.com/kyma-incubator/reconciler/issues/668
func (c *cleaner) purgeReconciliationsNew(transition *ClusterStatusTransition, config *CleanerConfig) {

	clusters, err := transition.inventory.GetAll()
	if err != nil {
		c.logger.Errorf("[CLEANER] Failed to get all clusters: %s", err.Error())
		return
	}

	for _, cluster := range clusters {
		c.purgeReconciliationsForCluster(cluster.Cluster.RuntimeID, transition, config)
	}

}

func (c *cleaner) purgeReconciliationsForCluster(runtimeID string, transition *ClusterStatusTransition, config *CleanerConfig) {
	c.logger.Infof("[Cleaner] Cleaning reconciliation entries for cluster with RuntimeID: %s", runtimeID)

	//1. Bulk delete old records, keeping the most recent one (should never be deleted)
	if err := c.deleteRecordsByAge(runtimeID, config.maxEntitiesAgeDays(), transition); err != nil {
		c.logger.Errorf("[CLEANER] Failed to delete reconciliations older than %d days: %s", config.maxEntitiesAgeDays(), err.Error())
		return
	}

	//2. Delete remaining records according to "count" and "status" criteria.
	if err := c.deleteRecordsByCountAndStatus(runtimeID, transition, config); err != nil {
		c.logger.Errorf("[CLEANER] Failed to delete reconciliations more than %d: %s", config.keepLatestEntitiesCount(), err.Error())
		return
	}
}

//deleteRecordsByAge deletes all reconciliations for a given cluster that's older than configured number of days except the single most recent record - that one is never deleted
func (c *cleaner) deleteRecordsByAge(runtimeID string, numberOfDays int, transition *ClusterStatusTransition) error {
	//TODO: Replace with bulk delete function (delete with filter) once it's added to the reconciliation.Repository interface

	now := time.Now()
	deadline := beginningOfTheDay(now.UTC()).AddDate(0, 0, -1*numberOfDays)

	c.logger.Infof("[CLEANER] Removing reconciliations older than: %s except the most recent one for the cluster %s", deadline.UTC().String(), runtimeID)

	mostRecentReconciliation, err := c.getLatestReconciliation(runtimeID, transition)
	if err != nil {
		return err
	}
	if mostRecentReconciliation == nil {
		c.logger.Infof("[CLEANER] No reconciliations found for the cluster: %s", runtimeID)
		return nil
	}

	removeExceptOneFilter := func(m *model.ReconciliationEntity) bool {
		return m.SchedulingID != mostRecentReconciliation.SchedulingID
	}

	return c.dropReconciliationsOlderThanByFilter(runtimeID, deadline, removeExceptOneFilter, transition)
}

//deleteRecordsByCount deletes record between some deadline in the past and now. It keeps the config.KeepLatestEntitiesCount() of the most recent records and the ones that are not successfully finished.
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

	reconciliationsToDrop := []*model.ReconciliationEntity{}
	for i := mostRecentEntitiesToKeep; i < len(reconciliations); i++ {
		if reconciliations[i].Status.IsFinalStable() {
			reconciliationsToDrop = append(reconciliationsToDrop, reconciliations[i])
		}
	}

	if len(reconciliationsToDrop) > 0 {
		c.logger.Debugf("[CLEANER] Found %d records with a \"successfull\" status to delete for the cluster %s", len(reconciliationsToDrop), runtimeID)
		c.removeReconciliations(reconciliationsToDrop, transition)
	}

	return nil
}

func (c *cleaner) getLatestReconciliation(runtimeID string, transition *ClusterStatusTransition) (*model.ReconciliationEntity, error) {
	limitFilter := reconciliation.Limit{Count: 1}
	runtimeIDFilter := reconciliation.WithRuntimeID{runtimeID}

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

func (c *cleaner) dropReconciliationsOlderThanByFilter(runtimeID string, t time.Time, shouldRemoveFilter reconciliationFilter, transition *ClusterStatusTransition) error {
	deadline := t.UTC()
	list, err := c.findReconciliationsOlderThan(runtimeID, deadline, transition)
	if err != nil {
		return err
	}

	filteredList := []*model.ReconciliationEntity{}

	for _, r := range list {
		if shouldRemoveFilter(r) {
			filteredList = append(filteredList, r)
		}
	}

	c.logger.Infof("[CLEANER] Found %d records older than %s for cluster %s", len(list), deadline.String(), runtimeID)
	if len(list) > 0 {
		c.removeReconciliations(filteredList, transition)
	}
	return err
}

func (c *cleaner) findReconciliationsOlderThan(runtimeID string, t time.Time, transition *ClusterStatusTransition) ([]*model.ReconciliationEntity, error) {

	runtimeIDFilter := reconciliation.WithRuntimeID{runtimeID}
	dateBeforeFilter := reconciliation.WithCreationDateBefore{t}

	filter := reconciliation.FilterMixer{Filters: []reconciliation.Filter{&runtimeIDFilter, &dateBeforeFilter}}

	reconciliations, err := transition.ReconciliationRepository().GetReconciliations(&filter)
	if err != nil {
		c.logger.Errorf("[CLEANER] Failed to get reconciliations older than %s: %s", t.String(), err.Error())
		return nil, err
	}
	return reconciliations, nil
}

//removeReconciliations drops all reconciliations provided in the list
func (c *cleaner) removeReconciliations(list []*model.ReconciliationEntity, transition *ClusterStatusTransition) {
	cnt := 0
	for _, r := range list {
		id := r.SchedulingID

		//err := transition.ReconciliationRepository().RemoveReconciliation(id)
		var err error = nil
		c.logger.Infof("[DEBUG] transition.ReconciliationRepository().RemoveReconciliation(%s)", id)
		c.logger.Infof("[DEBUG] removing reconciliation %s for a cluster: %s", r.SchedulingID, r.RuntimeID)
		if err == nil {
			cnt++
			if cnt%100 == 0 {
				c.logger.Infof("[CLEANER] Removed %d entities", cnt)
			}
		} else {
			c.logger.Errorf("[CLEANER] Failed to remove reconciliation with schedulingID '%s': %s", id, err.Error())
		}

	}
	c.logger.Infof("[CLEANER] Removed %d entities (finished)", cnt)
}

func (c *cleaner) purgeReconciliationsOld(transition *ClusterStatusTransition, config *CleanerConfig) {
	deadline := time.Now().UTC().Add(-1 * config.PurgeEntitiesOlderThan)
	reconciliations, err := transition.ReconciliationRepository().GetReconciliations(&reconciliation.WithCreationDateBefore{
		Time: deadline,
	})
	if err != nil {
		c.logger.Errorf("[CLEANER] Failed to get reconciliations older than %s: %s", deadline.String(), err.Error())
	}

	for i := range reconciliations {
		c.logger.Infof("[CLEANER] Is triggered for the Reconciliation and dependent Operations with SchedulingID '%s' "+
			"(created: %s)", reconciliations[i].SchedulingID, reconciliations[i].Created)

		id := reconciliations[i].SchedulingID
		//err := transition.ReconciliationRepository().RemoveReconciliation(id)
		c.logger.Infof("[DEBUG] transition.ReconciliationRepository().RemoveReconciliation(%s)", id)
		//if err != nil {
		//c.logger.Errorf("Cleaner failed to remove reconciliation with schedulingID '%s': %s", id, err.Error())
		//}
	}
}

//beginningOfTheDay returns t truncated to the very beginning of the day
func beginningOfTheDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

type reconciliationFilter func(*model.ReconciliationEntity) bool
