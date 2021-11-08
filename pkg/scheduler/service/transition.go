package service

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/pkg/errors"
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
		//set cluster status to reconciling or deleting depending on previous state
		var targetState model.Status
		if clusterState.Status.Status.IsDeleteCandidate() {
			targetState = model.ClusterStatusDeleting
		} else if clusterState.Status.Status.IsReconcileCandidate() {
			targetState = model.ClusterStatusReconciling
		} else {
			return fmt.Errorf("cannot start reconciliation of cluster %s because cluster is in state '%s'",
				clusterState.Cluster.RuntimeID, clusterState.Status.Status)
		}

		newClusterState, err := t.inventory.UpdateStatus(clusterState, targetState)
		if err == nil {
			t.logger.Debugf("Starting reconciliation for cluster '%s': set cluster status to '%s'",
				newClusterState.Cluster.RuntimeID, model.ClusterStatusReconciling)
		} else {
			t.logger.Errorf("Starting reconciliation for cluster '%s' failed: could not update cluster status to '%s': %s",
				clusterState.Cluster.RuntimeID, targetState, err)
			return err
		}

		//create reconciliation entity
		reconEntity, err := t.reconRepo.CreateReconciliation(newClusterState, preComponents)
		if err == nil {
			t.logger.Debugf("Starting reconciliation for cluster '%s' succeeded: reconciliation successfully enqueued "+
				"(reconciliation entity: %s)", newClusterState.Cluster.RuntimeID, reconEntity)
		} else {
			if reconciliation.IsEmptyComponentsReconciliationError(err) {
				t.logger.Errorf("Cluster transition tried to add cluster '%s' to reconciliation queue but "+
					"cluster has no components", newClusterState.Cluster.RuntimeID)
				_, updateErr := t.inventory.UpdateStatus(newClusterState, model.ClusterStatusReconcileError)
				if updateErr != nil {
					t.logger.Errorf("Error updating cluster '%s': could not update cluster status to '%s': %s",
						clusterState.Cluster.RuntimeID, model.ClusterStatusReconcileError, updateErr)
					return errors.Wrap(updateErr, err.Error())
				}
				return err
			}
			if reconciliation.IsDuplicateClusterReconciliationError(err) {
				t.logger.Debugf("Cancelling reconciliation for cluster '%s': cluster is already enqueued",
					newClusterState.Cluster.RuntimeID)
			} else {
				t.logger.Errorf("Starting reconciliation for runtime '%s' failed: "+
					"could not add runtime to reconciliation queue: %s", newClusterState.Cluster.RuntimeID, err)
			}
		}

		return err
	}
	return db.Transaction(t.conn, dbOp, t.logger)
}

func (t *ClusterStatusTransition) FinishReconciliation(schedulingID string, status model.Status) error {
	dbOp := func() error {
		reconEntity, err := t.reconRepo.GetReconciliation(schedulingID)
		if err != nil {
			t.logger.Errorf("Finishing reconciliation failed: could not retrieve reconciliation entity "+
				"(schedulingID:%s): %s", schedulingID, err)
			return err
		}

		if reconEntity.Finished {
			t.logger.Debugf("Finishing reconciliation for cluster '%s' failed: reconciliation entity (schedulingID:%s) "+
				"is already finished (maybe finished by parallel process in between)",
				reconEntity.RuntimeID, reconEntity.SchedulingID)
			return fmt.Errorf("failed to finish reconciliation '%s': it is already finished", reconEntity)
		}

		clusterState, err := t.inventory.Get(reconEntity.RuntimeID, reconEntity.ClusterConfig)
		if err != nil {
			t.logger.Errorf("Finishing reconciliation for cluster '%s' failed: could not get cluster state "+
				"(configVersion: %d): %s", reconEntity.RuntimeID, reconEntity.ClusterConfig, err)
			return err
		}

		clusterState, err = t.inventory.UpdateStatus(clusterState, status)
		if err != nil {
			t.logger.Errorf("Finishing reconciliation for cluster '%s' failed: "+
				"could not update cluster status to '%s': %s", clusterState.Cluster.RuntimeID, status, err)
			return err
		}

		err = t.reconRepo.FinishReconciliation(schedulingID, clusterState.Status)
		if err == nil {
			t.logger.Debugf("Finishing reconciliation for cluster '%s' succeeded "+
				"(schedulingID:%s/clusterVersion:%d/configVersion:%d): "+
				"new cluster status is '%s'", clusterState.Cluster.RuntimeID, schedulingID,
				clusterState.Cluster.Version, clusterState.Configuration.Version, clusterState.Status.Status)
		} else {
			t.logger.Errorf("Finishing reconciliation for cluster '%s' failed "+
				"(schedulingID:%s/clusterVersion:%d/configVersion:%d) : %s",
				clusterState.Cluster.RuntimeID, schedulingID,
				clusterState.Cluster.Version, clusterState.Configuration.Version, err)
			return err
		}

		if status == model.ClusterStatusDeleted {
			return t.inventory.Delete(clusterState.Cluster.RuntimeID)
		}
		return nil
	}
	return db.Transaction(t.conn, dbOp, t.logger)
}
