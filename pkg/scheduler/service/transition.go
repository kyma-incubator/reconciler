package service

import (
	"fmt"
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

func newClusterStatusTransition(
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
		//set cluster status to reconciling
		newClusterState, err := t.inventory.UpdateStatus(clusterState, model.ClusterStatusReconciling)
		if err == nil {
			t.logger.Debugf("Cluster transition set status of runtime '%s' to '%s' (cluster status entity: %s)",
				newClusterState.Cluster.RuntimeID, model.ClusterStatusReconciling, newClusterState.Status)
		} else {
			t.logger.Errorf("Cluster transition failed to update status of runtime '%s' to '%s': %s",
				clusterState.Cluster.RuntimeID, model.ClusterStatusReconciling, err)
		}

		//create reconciliation entity
		reconEntity, err := t.reconRepo.CreateReconciliation(newClusterState, preComponents)
		if err == nil {
			t.logger.Infof("Cluster transition finished: runtime '%s' added to reconciliation queue (reconciliation entity: %s)",
				newClusterState.Cluster.RuntimeID, reconEntity)
		} else {
			if reconciliation.IsDuplicateClusterReconciliationError(err) {
				t.logger.Infof("Cluster transition tried to add cluster '%s' to reconciliation queue but "+
					"cluster was already enqueued", newClusterState.Cluster.RuntimeID)
				return err
			}
			t.logger.Errorf("Cluster transition failed to add cluster '%s' to reconciliation queue: %s",
				newClusterState.Cluster.RuntimeID, err)
			return err
		}

		return err
	}
	return db.Transaction(t.conn, dbOp, t.logger)
}

func (t *ClusterStatusTransition) FinishReconciliation(schedulingID string, status model.Status) error {
	dbOp := func() error {
		reconEntity, err := t.reconRepo.GetReconciliation(schedulingID)
		if err != nil {
			t.logger.Errorf("Cluster transition failed to retrieve reconciliation entity with schedulingID '%s': %s",
				schedulingID, err)
			return err
		}

		if reconEntity.Finished {
			t.logger.Infof("Cluster transition tried to finish reconciliation '%s' but it is no longer marked to be in progress "+
				"(maybe finished by parallel process in between)", reconEntity)
			return fmt.Errorf("reconciliation '%s' is already finished", reconEntity)
		}

		clusterState, err := t.inventory.Get(reconEntity.RuntimeID, reconEntity.ClusterConfig)
		if err != nil {
			t.logger.Errorf("Cluster transition failed to retrieve cluster state for cluster '%s' "+
				"(configVersion: %d): %s", reconEntity.RuntimeID, reconEntity.ClusterConfig, err)
			return err
		}
		clusterState, err = t.inventory.UpdateStatus(clusterState, status)
		if err != nil {
			t.logger.Errorf("Cluster transition failed to update status of runtime '%s' to '%s': %s",
				clusterState.Cluster.RuntimeID, status, err)
			return err
		}
		err = t.reconRepo.FinishReconciliation(schedulingID, clusterState.Status)
		if err == nil {
			t.logger.Debugf("Cluster transition finished reconciliation (schedulingID '%s') of runtime '%s' (cluster-version %d / config-version %d): "+
				"new cluster status is '%s'", schedulingID, clusterState.Cluster.RuntimeID,
				clusterState.Cluster.Version, clusterState.Configuration.Version, clusterState.Status.Status)
		} else {
			t.logger.Errorf("Cluster transition failed to finish reconciliation (schedulingID '%s') of runtime '%s' "+
				"(cluster-version %d / config-version %d): %s",
				schedulingID, clusterState.Cluster.RuntimeID,
				clusterState.Cluster.Version, clusterState.Configuration.Version, err)
		}
		return err
	}
	return db.Transaction(t.conn, dbOp, t.logger)
}
