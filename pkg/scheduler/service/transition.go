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

func (t *ClusterStatusTransition) StartReconciliation(runtimeID string, configVersion int64, cfg *SchedulerConfig) error {
	var oldClusterState *cluster.State
	var newClusterState *cluster.State
	dbOp := func(tx *db.TxConnection) error {
		inventoryTx, err := t.inventory.WithTx(tx)
		if err != nil {
			return err
		}

		reconRepoTx, err := t.reconRepo.WithTx(tx)
		if err != nil {
			return err
		}

		recons, err := reconRepoTx.GetReconciliations(&reconciliation.CurrentlyReconcilingWithRuntimeID{
			RuntimeID: runtimeID,
		})
		if err != nil {
			return errors.Wrapf(err, "failed to retrieve reconciliations for runtimeID '%s'", runtimeID)
		}
		if len(recons) > 0 {
			return fmt.Errorf("cannot start reconciliation for cluster '%s': cluster is already enqueued "+
				"with schedulingID '%s'", runtimeID, recons[0].SchedulingID)
		}

		oldClusterState, err = inventoryTx.Get(runtimeID, configVersion)
		if err != nil {
			t.logger.Errorf("Starting reconciliation for cluster '%s' failed: could not get latest cluster state: %s",
				runtimeID, err)
			return err
		}

		//set cluster status to reconciling or deleting depending on previous state
		var targetState model.Status
		if oldClusterState.Status.Status.IsDeleteCandidate() {
			targetState = model.ClusterStatusDeleting
		} else if oldClusterState.Status.Status.IsReconcileCandidate() {
			targetState = model.ClusterStatusReconciling
		} else {
			return fmt.Errorf("cannot start reconciliation of cluster %s because cluster is in state '%s'",
				oldClusterState.Cluster.RuntimeID, oldClusterState.Status.Status)
		}

		newClusterState, err = inventoryTx.UpdateStatus(oldClusterState, targetState)
		if err != nil {
			t.logger.Errorf("Starting reconciliation for cluster '%s' failed: could not update cluster status to '%s': %s",
				oldClusterState.Cluster.RuntimeID, targetState, err)
			return err
		}
		t.logger.Debugf("Starting reconciliation for cluster '%s': set cluster status to '%s'",
			newClusterState.Cluster.RuntimeID, newClusterState.Status.Status)

		//create reconciliation entity
		reconEntity, err := reconRepoTx.CreateReconciliation(newClusterState, &model.ReconciliationSequenceConfig{
			PreComponents:        cfg.PreComponents,
			DeleteStrategy:       string(cfg.DeleteStrategy),
			ReconciliationStatus: newClusterState.Status.Status,
		})
		if err == nil {
			t.logger.Infof("Starting reconciliation for cluster '%s' succeeded: reconciliation successfully enqueued "+
				"(scheudlingID: %s)", newClusterState.Cluster.RuntimeID, reconEntity.SchedulingID)
			return nil
		}

		//sort ouf if issue is caused by a race condition (just for logging purpose)
		if reconciliation.IsDuplicateClusterReconciliationError(err) {
			t.logger.Infof("Cancelling reconciliation for cluster '%s': cluster is already enqueued (race condition)",
				newClusterState.Cluster.RuntimeID)
		} else {
			t.logger.Errorf("Starting reconciliation for runtime '%s' failed: "+
				"could not add runtime to reconciliation queue: %s", newClusterState.Cluster.RuntimeID, err)
		}

		return err
	}
	err := db.Transaction(t.conn, dbOp, t.logger)
	if reconciliation.IsEmptyComponentsReconciliationError(err) {
		t.logger.Errorf("Cluster transition tried to add cluster '%s' to reconciliation queue but "+
			"cluster has no components", newClusterState.Cluster.RuntimeID)
		_, updateErr := t.inventory.UpdateStatus(newClusterState, model.ClusterStatusReconcileError)
		if updateErr != nil {
			t.logger.Errorf("Error updating cluster '%s': could not update cluster status to '%s': %s",
				oldClusterState.Cluster.RuntimeID, model.ClusterStatusReconcileError, updateErr)
			return errors.Wrap(updateErr, err.Error())
		}
	}
	return err
}

func (t *ClusterStatusTransition) FinishReconciliation(schedulingID string, status model.Status) error {
	dbOp := func(tx *db.TxConnection) error {
		inventory, err := t.inventory.WithTx(tx)
		if err != nil {
			return err
		}

		reconRepo, err := t.reconRepo.WithTx(tx)
		if err != nil {
			return err
		}

		reconEntity, err := reconRepo.GetReconciliation(schedulingID)
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

		clusterState, err := inventory.Get(reconEntity.RuntimeID, reconEntity.ClusterConfig)
		if err != nil {
			t.logger.Errorf("Finishing reconciliation for cluster '%s' failed: could not get cluster state : %s", reconEntity.RuntimeID, err)
			return err
		}

		if clusterState.Status.Status.IsInProgress() {
			oldClusterStatus := clusterState.Status.Status
			clusterState, err = inventory.UpdateStatus(clusterState, status)
			if err != nil {
				t.logger.Errorf("Finishing reconciliation for cluster '%s' failed: "+
					"could not update cluster status from %s to '%s': %s", clusterState.Cluster.RuntimeID, oldClusterStatus, status, err)
				return err
			}
		} else {
			t.logger.Warnf("Finishing reconciliation for cluster '%s': skipped cluster status update: current[%s], target[%s]"+
				"(schedulingID:%s/clusterVersion:%d/configVersion:%d)",
				clusterState.Cluster.RuntimeID, clusterState.Status.Status, status,
				schedulingID, clusterState.Cluster.Version, clusterState.Configuration.Version)
		}

		err = reconRepo.FinishReconciliation(schedulingID, clusterState.Status)
		if err == nil {
			t.logger.Infof("Finishing reconciliation for cluster '%s' succeeded "+
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
			return inventory.Delete(clusterState.Cluster.RuntimeID)
		}
		return nil
	}
	return db.Transaction(t.conn, dbOp, t.logger)
}
