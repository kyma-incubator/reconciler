package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
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
)

// set debugLogging to control the debug logging for all components in the test
const debugLogging = true

type customAction struct {
	success bool
}

func (a *customAction) Run(_ *service.ActionContext) error {
	if a.success {
		return nil
	}
	return fmt.Errorf("action failed")
}

func TestRuntimeBuilder(t *testing.T) {
	test.IntegrationTest(t)

	//register custom 'base' component reconciler for this unittest
	compRecon, err := service.NewComponentReconciler("base")
	require.NoError(t, err)
	compRecon.WithRetry(1, 1*time.Second)

	t.Run("Run local with success (waiting for CRDs)", func(t *testing.T) {
		compRecon.WithReconcileAction(&customAction{true})
		reconResult, receivedUpdates := runLocal(t, 30*time.Second)
		require.Equal(t, model.ClusterStatusReady, reconResult.GetResult())
		require.Equal(t, reconciler.StatusSuccess, receivedUpdates[len(receivedUpdates)-1].Status)
	})

	t.Run("Run local with error", func(t *testing.T) {
		compRecon.WithReconcileAction(&customAction{false})
		reconResult, receivedUpdates := runLocal(t, 5*time.Second)
		require.Equal(t, model.ClusterStatusReconcileError, reconResult.GetResult())
		require.Equal(t, reconciler.StatusError, receivedUpdates[len(receivedUpdates)-1].Status)
	})

	t.Run("Run remote with success", func(t *testing.T) {
		runRemote(t, model.ClusterStatusReady, 30*time.Second)
	})

	t.Run("Run remote with error", func(t *testing.T) {
		runRemote(t, model.ClusterStatusReconcileError, 5*time.Second)
	})

}

func runRemote(t *testing.T, expectedClusterStatus model.Status, timeout time.Duration) {
	dbConn := db.NewTestConnection(t)

	//create cluster entity
	inventory, err := cluster.NewInventory(dbConn, debugLogging, cluster.MetricsCollectorMock{})
	require.NoError(t, err)
	clusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
		Kubeconfig: test.ReadKubeconfig(t),
		KymaConfig: keb.KymaConfig{
			Components: []keb.Component{
				{
					Component: "TestComp1",
					Namespace: "NS1",
				},
			},
			Profile: "",
			Version: "1.2.3",
		},
		RuntimeID: uuid.NewString(),
	})
	require.NoError(t, err)

	//create reconciliation repository
	reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, debugLogging)
	require.NoError(t, err)

	//cleanup
	defer func() {
		require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
		recons, err := reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: clusterState.Cluster.RuntimeID})
		require.NoError(t, err)
		for _, recon := range recons {
			require.NoError(t, reconRepo.RemoveReconciliation(recon.SchedulingID))
		}
	}()

	//configure remote runner
	runtimeBuilder := NewRuntimeBuilder(reconRepo, logger.NewLogger(debugLogging))
	remoteRunner := runtimeBuilder.RunRemote(dbConn, inventory, &config.Config{
		Scheme: "https",
		Host:   "httpbin.org",
		Port:   443,
		Scheduler: config.SchedulerConfig{
			PreComponents: []string{
				"dummyComponent",
			},
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

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	require.NoError(t, remoteRunner.Run(ctx))

	setOperationState(t, reconRepo, expectedClusterStatus, clusterState.Cluster.RuntimeID)

	time.Sleep(3 * time.Second) //give the bookkeeper some time to update the reconciliation

	newClusterState, err := inventory.GetLatest(clusterState.Cluster.RuntimeID)
	require.NoError(t, err)
	require.Equal(t, newClusterState.Status.Status, expectedClusterStatus)
}

//setOperationState will update all operation status accordingly to expected cluster state
func setOperationState(t *testing.T, reconRepo reconciliation.Repository, expectedClusterStatus model.Status, runtimeID string) {
	var opState model.OperationState
	switch expectedClusterStatus {
	case model.ClusterStatusReconcileError:
		opState = model.OperationStateError
	case model.ClusterStatusReady:
		opState = model.OperationStateDone
	default:
		t.Logf("Cannot map cluster state '%s' to an operation state", expectedClusterStatus)
		t.FailNow()
	}

	//simulate a successfully finished operation
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		//try to get the reconciliation entity for the cluster
		reconEntities, err := reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: runtimeID})
		if len(reconEntities) == 0 {
			continue
		}
		require.NoError(t, err)
		require.Len(t, reconEntities, 1)

		//get operations of this reconciliation
		opEntities, err := reconRepo.GetOperations(reconEntities[0].SchedulingID)
		require.NoError(t, err)

		//set all operations to a final state
		for _, opEntity := range opEntities {
			err = reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID,
				opState, "dummy reason")

			if err != nil { //probably a race condition (because invoker is updating ops-states in background as well)
				latestOpEntity, errGetOp := reconRepo.GetOperation(opEntity.SchedulingID, opEntity.CorrelationID)
				require.NoError(t, errGetOp)
				t.Logf("Failed to updated operation state: %s -> latest operation state is '%s'. Will try again...",
					err, latestOpEntity.State)
				require.NoError(t, reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID,
					opState, "dummy reason"))
			}
		}
		return
	}
}

func runLocal(t *testing.T, timeout time.Duration) (*ReconciliationResult, []*reconciler.CallbackMessage) {
	//create cluster entity
	inventory, err := cluster.NewInventory(db.NewTestConnection(t), debugLogging, cluster.MetricsCollectorMock{})
	require.NoError(t, err)
	clusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
		Kubeconfig: test.ReadKubeconfig(t),
		KymaConfig: keb.KymaConfig{
			Components: []keb.Component{
				{
					Component: "TestComp1",
					Namespace: "NS1",
				},
			},
			Profile: "",
			Version: "1.2.3",
		},
		RuntimeID: uuid.NewString(),
	})
	require.NoError(t, err)

	//cleanup
	defer func() {
		require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
	}()

	//create reconciliation repository
	reconRepo := reconciliation.NewInMemoryReconciliationRepository()

	//configure local runner
	var receivedUpdates []*reconciler.CallbackMessage
	runtimeBuilder := NewRuntimeBuilder(reconRepo, logger.NewLogger(debugLogging))
	localRunner := runtimeBuilder.RunLocal(nil, func(component string, msg *reconciler.CallbackMessage) {
		receivedUpdates = append(receivedUpdates, msg)
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	reconResult, err := localRunner.Run(ctx, clusterState)
	require.NoError(t, err)

	return reconResult, receivedUpdates
}
