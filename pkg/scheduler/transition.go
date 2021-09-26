package scheduler

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"go.uber.org/zap"
)

type ClusterStatusTransition struct {
	conn      db.Connection
	inventory cluster.Inventory
	reconRepo reconciliation.Repository
	logger    *zap.SugaredLogger
}

func NewClusterStatusTransition(
	conn db.Connection,
	inventory cluster.Inventory,
	reconRepo reconciliation.Repository,
	logger *zap.SugaredLogger) *ClusterStatusTransition {
	return &ClusterStatusTransition{
		conn:      conn,
		inventory: inventory,
		reconRepo: reconRepo,
		logger:    logger,
	}
}

func (t *ClusterStatusTransition) Inventory() cluster.Inventory {
	return t.inventory
}

func (t *ClusterStatusTransition) ReconciliationRepository() reconciliation.Repository {
	return t.reconRepo
}

func (t *ClusterStatusTransition) StartReconciliation(clusterState *cluster.State, preComponents []string) error {
	dbOp := func() error {
		//create reconciliation entity
		reconEntity, err := t.reconRepo.CreateReconciliation(clusterState, preComponents)
		if err != nil {
			if reconciliation.IsDuplicateClusterReconciliationError(err) {
				t.logger.Infof("Tried to add cluster '%s' to reconciliation queue but "+
					"cluster was already enqueued", clusterState.Cluster.Cluster)
				return nil
			} else {
				t.logger.Errorf("Failed to add cluster '%s' to reconciliation queue: %s",
					clusterState.Cluster.Cluster, err)
				return err
			}
		}
		//set cluster status to reconciling
		_, err = t.inventory.UpdateStatus(clusterState, model.ClusterStatusReconciling)
		if err == nil {
			t.logger.Debugf("Added cluster '%s' to reconcilication queue (reconciliation entity: %s)",
				clusterState.Cluster.Cluster, reconEntity)
			t.logger.Debugf("Set status of cluster '%s' to '%s'",
				clusterState.Cluster.Cluster, model.ClusterStatusReconciling)
		} else {
			t.logger.Errorf("Failed to update status of cluster '%s' to '%s': %s",
				clusterState.Cluster.Cluster, model.ClusterStatusReconciling, err)
		}

		return err
	}
	return db.Transaction(t.conn, dbOp, t.logger)
}

func (t *ClusterStatusTransition) FinishReconciliation(schedulingID string, status model.Status) error {
	dbOp := func() error {
		reconEntity, err := t.reconRepo.GetReconciliation(schedulingID)
		if err != nil {
			t.logger.Errorf("Failed to retrieve reconciliation entity with schedulingID '%s': %s",
				schedulingID, err)
			return err
		}
		clusterState, err := t.inventory.Get(reconEntity.Cluster, reconEntity.ClusterConfig)
		if err != nil {
			t.logger.Errorf("Failed to retrieve cluster state for cluster '%s' (configVersion: %d): %s",
				reconEntity.Cluster, reconEntity.ClusterConfig, err)
			return err
		}
		clusterState, err = t.inventory.UpdateStatus(clusterState, status)
		if err != nil {
			t.logger.Errorf("Failed to update status of cluster '%s' to '%s': %s",
				clusterState.Cluster.Cluster, status, err)
			return err
		}
		err = t.reconRepo.FinishReconciliation(schedulingID, clusterState.Status)
		if err == nil {
			t.logger.Debugf("Reconciliation of cluster '%s' (schedulingID '%s') finished: new cluster status is '%s'",
				clusterState.Cluster.Cluster, schedulingID, clusterState.Status.Status)
		} else {
			t.logger.Errorf("Failed to finish reconciliation with schedulingID '%s' of cluster '%s': %s",
				schedulingID, clusterState.Cluster.Cluster, err)
		}
		return err
	}
	return db.Transaction(t.conn, dbOp, t.logger)
}
