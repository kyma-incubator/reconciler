package service

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/worker"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type customAction struct {
	success bool
}

func (a *customAction) Run(_, _ string, _ map[string]interface{}, _ *service.ActionContext) error {
	if a.success {
		return nil
	}
	return fmt.Errorf("action failed")
}

func TestRuntimeBuilder(t *testing.T) {
	test.IntegrationTest(t)

	//register custom 'base' component reconciler for this unitest
	compRecon, err := service.NewComponentReconciler("base")
	require.NoError(t, err)
	compRecon.WithRetry(1, 1*time.Second)

	t.Run("Run local with success", func(t *testing.T) {
		compRecon.WithReconcileAction(&customAction{true})
		receivedUpdates := runLocal(t)
		require.Equal(t, receivedUpdates[len(receivedUpdates)-1].Status, reconciler.StatusSuccess)
	})

	t.Run("Run local with error", func(t *testing.T) {
		compRecon.WithReconcileAction(&customAction{false})
		receivedUpdates := runLocal(t)
		require.Equal(t, receivedUpdates[len(receivedUpdates)-1].Status, reconciler.StatusError)
	})

	t.Run("Run remote with success", func(t *testing.T) {
		runRemote(t, model.ClusterStatusReady)
	})

	t.Run("Run remote with error", func(t *testing.T) {
		runRemote(t, model.ClusterStatusError)
	})

}

func runRemote(t *testing.T, expectedClusterStatus model.Status) {
	dbConn := db.NewTestConnection(t)

	//create cluster entity
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)
	clusterState, err = inventory.CreateOrUpdate(1, &keb.Cluster{
		Kubeconfig: test.ReadKubeconfig(t),
		KymaConfig: keb.KymaConfig{
			Components: nil,
			Profile:    "",
			Version:    "1.2.3",
		},
		RuntimeID: "testCluster",
	})
	require.NoError(t, err)

	//create reconciliation repository
	reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, true)
	require.NoError(t, err)

	//configure remote runner
	runtimeBuilder := NewRuntimeBuilder(reconRepo, logger.NewLogger(true))
	remoteRunner := runtimeBuilder.RunRemote(dbConn, inventory, &config.Config{
		Scheme: "https",
		Host:   "httpbin.org",
		Port:   443,
		Scheduler: config.SchedulerConfig{
			PreComponents: nil,
			Reconcilers: map[string]config.ComponentReconciler{
				"base": {
					URL: "https://httpbin.org/post",
				},
			},
		},
	})
	remoteRunner.WithBookkeeperConfig(&BookkeeperConfig{
		OperationsWatchInterval: 1 * time.Second,
		OrphanOperationTimeout:  10 * time.Second,
	})
	remoteRunner.WithWorkerPoolConfig(&worker.Config{
		PoolSize:               10,
		OperationCheckInterval: 1 * time.Second,
		InvokerMaxRetries:      2,
		InvokerRetryDelay:      2 * time.Second,
	})
	remoteRunner.WithSchedulerConfig(&SchedulerConfig{
		InventoryWatchInterval:   1 * time.Second,
		ClusterReconcileInterval: 1 * time.Minute,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second) //abort runner latest after 10 sec
	defer cancel()
	remoteRunner.Run(ctx)

	setOperationState(t, reconRepo, expectedClusterStatus)

	time.Sleep(3 * time.Second) //give the bookkeeper some time to update the reconciliation

	newClusterState, err := inventory.GetLatest(clusterState.Cluster.Cluster)
	require.NoError(t, err)
	require.Equal(t, newClusterState.Status.Status, expectedClusterStatus)
}

//setOperationState will update all operation status accordingly to expected cluster state
func setOperationState(t *testing.T, reconRepo reconciliation.Repository, expectedClusterStatus model.Status) {
	var opState model.OperationState
	switch expectedClusterStatus {
	case model.ClusterStatusError:
		opState = model.OperationStateError
	case model.ClusterStatusReady:
		opState = model.OperationStateDone
	default:
		t.Logf("Cannot map cluster state '%s' to an operation state", expectedClusterStatus)
		t.FailNow()
	}

	//simulate a successfully finished operation
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			//try to get the reconciliation entity for the cluster
			reconEntities, err := reconRepo.GetReconciliations(&reconciliation.WithCluster{Cluster: clusterState.Cluster.Cluster})
			if len(reconEntities) == 0 {
				continue
			}
			require.NoError(t, err)
			require.Len(t, reconEntities, 1)

			//get operations of this reconciliation
			opEntities, err := reconRepo.GetOperations(reconEntities[0].SchedulingID)
			require.NoError(t, err)
			//set all operations to DONE
			for _, opEntity := range opEntities {
				t.Log("Updating operations to state DONE")
				err = reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID,
					opState, "dummy reason")
				require.NoError(t, err)
			}
			return
		}
	}
}

func runLocal(t *testing.T) []*reconciler.CallbackMessage {
	//create cluster entity
	inventory, err := cluster.NewInventory(db.NewTestConnection(t), true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)
	clusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
		Kubeconfig: test.ReadKubeconfig(t),
		KymaConfig: keb.KymaConfig{
			Components: nil,
			Profile:    "",
			Version:    "1.2.3",
		},
		RuntimeID: "testCluster",
	})
	require.NoError(t, err)

	//create reconciliation repository
	reconRepo := reconciliation.NewInMemoryReconciliationRepository()

	//configure local runner
	var receivedUpdates []*reconciler.CallbackMessage
	runtimeBuilder := NewRuntimeBuilder(reconRepo, logger.NewLogger(true))
	localRunner := runtimeBuilder.RunLocal(nil, func(component string, msg *reconciler.CallbackMessage) {
		receivedUpdates = append(receivedUpdates, msg)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second) //abort runner latest after 5 sec
	defer cancel()

	err = localRunner.Run(ctx, clusterState)
	require.NoError(t, err)
	return receivedUpdates
}
