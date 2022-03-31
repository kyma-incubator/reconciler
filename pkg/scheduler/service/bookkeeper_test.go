package service

import (
	"context"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
	"github.com/stretchr/testify/require"
)

func (s *reconciliationTestSuite) TestBookkeeper() {
	t := s.T()
	dbConn := s.TxConnection() //share one db-connection between inventory and recon-repo (required for tx)

	//prepare inventory
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)
	clusterState, err := inventory.CreateOrUpdate(1, test.NewCluster(t, "1", 1, false, test.OneComponentDummy))
	require.NoError(t, err)

	//trigger reconciliation for cluster
	reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, true)
	require.NoError(t, err)
	reconEntity, err := reconRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})
	require.NoError(t, err)
	require.NotEmpty(t, reconEntity.Lock)
	require.False(t, reconEntity.Finished)

	//mark all operations to be finished
	opEntities, err := reconRepo.GetOperations(&operation.WithSchedulingID{
		SchedulingID: reconEntity.SchedulingID,
	})
	require.NoError(t, err)
	for _, opEntity := range opEntities {
		err := reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID, model.OperationStateDone, true)
		require.NoError(t, err)
	}

	//initialize bookkeeper
	bk := newBookkeeper(
		reconRepo,
		&BookkeeperConfig{
			OperationsWatchInterval: 1 * time.Second,
			OrphanOperationTimeout:  2 * time.Second,
		},
		logger.NewLogger(true),
	)

	//run bookkeeper
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //stop bookkeeper after 5 sec
	defer cancel()

	transition := newClusterStatusTransition(dbConn, inventory, reconRepo, logger.NewLogger(true))
	start := time.Now()
	require.NoError(t, bk.Run(ctx,
		markOrphanOperation{transition: transition, logger: transition.logger},
		finishOperation{transition: transition, logger: transition.logger}))
	require.WithinDuration(t, time.Now(), start, 5500*time.Millisecond) //verify bookkeeper stops when ctx gets closed

	//verify bookkeeper results
	reconEntityUpdated, err := reconRepo.GetReconciliation(reconEntity.SchedulingID)
	require.NoError(t, err)
	require.True(t, reconEntityUpdated.Finished)

	//cleanup
	t.Log("Cleaning up test context")
	require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
	recons, err := reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID})
	require.NoError(t, err)
	for _, recon := range recons {
		require.NoError(t, reconRepo.RemoveReconciliationBySchedulingID(recon.SchedulingID))
	}
}
