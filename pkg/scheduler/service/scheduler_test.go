package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/stretchr/testify/require"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
)

func (s *serviceTestSuite) TestSchedulerRunOnce() {
	t := s.T()
	// run once will already expect the reconciliation to be in progress as there is no scheduling in place
	clusterState := testClusterState("testCluster", 1, model.ClusterStatusReconciling)
	reconRepo := reconciliation.NewInMemoryReconciliationRepository()
	scheduler := newScheduler(s.testLogger)
	require.NoError(t, scheduler.RunOnce(clusterState, reconRepo, &SchedulerConfig{}))
	requiredReconciliationEntity(t, reconRepo, clusterState)
}

func (s *serviceTestSuite) TestSchedulerRun() {
	t := s.T()
	reconRepo := reconciliation.NewInMemoryReconciliationRepository()
	scheduler := newScheduler(s.testLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()

	// for a regular run within the scheduling loop, the cluster to be processed must be in pending state first
	clusterState := testClusterState("testCluster", 1, model.ClusterStatusReconcilePending)

	err := scheduler.Run(ctx, &ClusterStatusTransition{
		conn: s.TxConnection(),
		inventory: &cluster.MockInventory{
			//this will cause the creation of a reconciliation for the same cluster multiple times:
			ClustersToReconcileResult: []*cluster.State{
				clusterState,
			},
			//simulate an updated cluster status (required when transition updates the cluster status)
			UpdateStatusResult: func() *cluster.State {
				// here we expect that after updating the cluster, it will set it to reconciling as it is no longer
				// waiting to be scheduled
				updatedState := testClusterState("testCluster", 2, model.ClusterStatusReconciling)
				return updatedState
			}(),
			GetResult: func() *cluster.State {
				return clusterState
			}(),
		},
		reconRepo: reconRepo,
		logger:    s.testLogger,
	}, &SchedulerConfig{
		InventoryWatchInterval:   250 * time.Millisecond,
		ClusterReconcileInterval: 100 * time.Second,
		ClusterQueueSize:         5,
	})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond) //give it some time to shutdown

	require.WithinDuration(t, start, time.Now(), 4*time.Second)
	requiredReconciliationEntity(t, reconRepo, clusterState)
}

func requiredReconciliationEntity(t *testing.T, reconRepo reconciliation.Repository, state *cluster.State) {
	recons, err := reconRepo.GetReconciliations(nil)
	require.NoError(t, err)
	require.Len(t, recons, 1)
	require.Equal(t, recons[0].RuntimeID, state.Cluster.RuntimeID)
	ops, err := reconRepo.GetOperations(&operation.WithSchedulingID{
		SchedulingID: recons[0].SchedulingID,
	})
	require.NoError(t, err)
	if state.Status.Status.IsDeletionInProgress() {
		require.Len(t, ops, 3) // cleaner is expected if reconciliation is a deletion
	} else {
		require.Len(t, ops, 2)
	}
	require.Equal(t, ops[0].RuntimeID, state.Cluster.RuntimeID)
}

func testClusterState(clusterID string, statusID int64, status model.Status) *cluster.State {
	return &cluster.State{
		Cluster: &model.ClusterEntity{
			Version:    1,
			RuntimeID:  clusterID,
			Kubeconfig: "xyz",
			Contract:   1,
		},
		Configuration: &model.ClusterConfigurationEntity{
			Version:        1,
			RuntimeID:      clusterID,
			ClusterVersion: 1,
			Contract:       1,
			KymaVersion:    "1.24.0",
			Components: []*keb.Component{
				{
					Component: "testComp1",
					Version:   "1",
				},
			},
		},
		Status: &model.ClusterStatusEntity{
			ID:             statusID,
			RuntimeID:      clusterID,
			ClusterVersion: 1,
			ConfigVersion:  1,
			Status:         status,
		},
	}
}

func createClusterStates(t *testing.T, inventory cluster.Inventory) []string {
	clusterID1 := uuid.NewString()
	s1, err := inventory.CreateOrUpdate(1, test.NewCluster(t, clusterID1, 1, false, test.ThreeComponentsDummy))
	require.NoError(t, err)

	clusterID2 := uuid.NewString()
	s2, err := inventory.CreateOrUpdate(1, test.NewCluster(t, clusterID2, 1, false, test.OneComponentDummy))
	require.NoError(t, err)

	return []string{s1.Cluster.RuntimeID, s2.Cluster.RuntimeID}
}

func (s *serviceTestSuite) TestMultipleSchedulerWatchingSameInventory() {
	t := s.T()
	dbConn, err := s.NewConnection()
	require.NoError(t, err)
	//initialize WaitGroup
	var wg sync.WaitGroup

	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	clusterRuntimeIDs := createClusterStates(t, inventory)

	scheduler := newScheduler(s.testLogger)
	reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, true)
	require.NoError(t, err)

	removeExistingReconciliations(t, reconRepo) //cleanup before test execution

	ctx, cancelFct := context.WithTimeout(context.Background(), 10*time.Second)
	defer func(dbConn db.Connection) {
		for _, runtimeID := range clusterRuntimeIDs {
			require.NoError(t, inventory.Delete(runtimeID))
		}
		cancelFct()
		removeExistingReconciliations(t, reconRepo)
		require.NoError(t, dbConn.Close())
	}(dbConn)

	require.Len(t, getReconciliations(t, clusterRuntimeIDs, reconRepo), 0) //ensure no reconciliation exist

	startAt := time.Now().Add(1 * time.Second)
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Until(startAt))
			err := scheduler.Run(ctx, &ClusterStatusTransition{
				conn:      dbConn,
				inventory: inventory,
				reconRepo: reconRepo,
				logger:    s.testLogger,
			}, &SchedulerConfig{
				InventoryWatchInterval:   100 * time.Millisecond,
				ClusterReconcileInterval: 100 * time.Second,
				ClusterQueueSize:         10,
			})
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	require.Len(t, getReconciliations(t, clusterRuntimeIDs, reconRepo), 2) //ensure reconciliations were created
}

func getReconciliations(t *testing.T, clusterRuntimeIDs []string, reconRepo reconciliation.Repository) []*model.ReconciliationEntity {
	cif := runtimeIDFilter{clusterRuntimeIDs}
	recons, err := reconRepo.GetReconciliations(&cif)
	require.NoError(t, err)
	return recons
}

func (s *serviceTestSuite) TestDeleteStrategy() {
	t := s.T()
	// Happy paths
	ds, err := NewDeleteStrategy("system")
	require.NoError(t, err)
	require.Equal(t, DeleteStrategySystem, ds)

	ds, err = NewDeleteStrategy("all")
	require.NoError(t, err)
	require.Equal(t, DeleteStrategyAll, ds)

	// Upper case resistant
	ds, err = NewDeleteStrategy("System")
	require.NoError(t, err)
	require.Equal(t, DeleteStrategySystem, ds)

	ds, err = NewDeleteStrategy("All")
	require.NoError(t, err)
	require.Equal(t, DeleteStrategyAll, ds)

	// empty value guard
	ds, err = NewDeleteStrategy("")
	require.NoError(t, err)
	require.Equal(t, DeleteStrategySystem, ds)

	// unsupported value
	_, err = NewDeleteStrategy("not-a-strategy")
	require.Error(t, err)
}

type runtimeIDFilter struct {
	clusterIDs []string
}

func (c *runtimeIDFilter) FilterByQuery(q *db.Select) error {
	q.WhereIn("RuntimeID", "$1, $2", c.clusterIDs[0], c.clusterIDs[1])
	return nil
}

func (c *runtimeIDFilter) FilterByInstance(i *model.ReconciliationEntity) *model.ReconciliationEntity {
	return i
}
