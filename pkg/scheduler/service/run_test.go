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
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
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
	compRecon.WithRetryDelay(1 * time.Second)

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

		//Because of parallel processing the message order is not guaranteed. Check the count of different statuses instead
		require.Equal(t, 1, countStatus(reconciler.StatusRunning, receivedUpdates))
		require.Equal(t, 1, countStatus(reconciler.StatusError, receivedUpdates))
		require.Equal(t, 1, countStatus(reconciler.StatusFailed, receivedUpdates))
	})

	t.Run("Run remote with success", func(t *testing.T) {
		runRemote(t, model.ClusterStatusReady, 30*time.Second)
	})

	t.Run("Run remote with error", func(t *testing.T) {
		runRemote(t, model.ClusterStatusReconcileErrorRetryable, 20*time.Second)
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
			PreComponents: [][]string{
				{"dummyComponent"},
			},
			Reconcilers: map[string]config.ComponentReconciler{
				"base": {
					URL: "https://httpbin.org/post",
				},
			},
		},
	})
	remoteRunner.WithBookkeeperConfig(&BookkeeperConfig{
		OperationsWatchInterval: 5 * time.Second,
		OrphanOperationTimeout:  10 * time.Second,
	})
	remoteRunner.WithWorkerPoolConfig(&worker.Config{
		PoolSize:               10,
		OperationCheckInterval: 5 * time.Second,
		InvokerMaxRetries:      2,
		InvokerRetryDelay:      2 * time.Second,
	})
	remoteRunner.WithSchedulerConfig(&SchedulerConfig{
		InventoryWatchInterval:   5 * time.Second,
		ClusterReconcileInterval: 1 * time.Minute,
	})
	remoteRunner.WithCleanerConfig(&CleanerConfig{
		PurgeEntitiesOlderThan: 15 * time.Second,
		CleanerInterval:        4 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	require.NoError(t, remoteRunner.Run(ctx))

	setOperationState(t, reconRepo, expectedClusterStatus, clusterState.Cluster.RuntimeID)

	time.Sleep(5 * time.Second) //give the bookkeeper some time to update the reconciliation

	newClusterState, err := inventory.GetLatest(clusterState.Cluster.RuntimeID)
	require.NoError(t, err)
	require.Equal(t, expectedClusterStatus, newClusterState.Status.Status)

	//verify that reconciliation was correctly created
	recons, err := reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: newClusterState.Cluster.RuntimeID})
	require.NoError(t, err)
	require.Len(t, recons, 1)

	schedulingID := recons[0].SchedulingID
	require.Equal(t, 3, countOperations(t, reconRepo, schedulingID))

	time.Sleep(15 * time.Second) //give the cleaner some time to remove old entities

	//check whether the cleaner was removing the entities properly
	recons, err = reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: newClusterState.Cluster.RuntimeID})
	require.NoError(t, err)
	require.Len(t, recons, 0)
	require.Equal(t, 0, countOperations(t, reconRepo, schedulingID))

}

func countOperations(t *testing.T, reconRepo reconciliation.Repository, schedulingID string) int {
	ops, err := reconRepo.GetOperations(&operation.WithSchedulingID{SchedulingID: schedulingID})
	require.NoError(t, err)
	return len(ops)
}

//setOperationState will update all operation status accordingly to expected cluster state
func setOperationState(t *testing.T, reconRepo reconciliation.Repository, expectedClusterStatus model.Status, runtimeID string) {
	var opState model.OperationState
	switch expectedClusterStatus {
	case model.ClusterStatusReconcileError:
		opState = model.OperationStateError
	case model.ClusterStatusReady:
		opState = model.OperationStateDone
	case model.ClusterStatusReconcileErrorRetryable:
		opState = model.OperationStateError
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
		opEntities, err := reconRepo.GetOperations(&operation.WithSchedulingID{
			SchedulingID: reconEntities[0].SchedulingID,
		})
		require.NoError(t, err)

		//set all operations to a final state
		for _, opEntity := range opEntities {
			err = reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID,
				opState, true, "dummy reason")

			if err != nil { //probably a race condition (because invoker is updating ops-states in background as well)
				latestOpEntity, errGetOp := reconRepo.GetOperation(opEntity.SchedulingID, opEntity.CorrelationID)
				require.NoError(t, errGetOp)
				t.Logf("Failed to updated operation state: %s -> latest operation state is '%s'. Will try again...",
					err, latestOpEntity.State)
				require.NoError(t, reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID,
					opState, true, "dummy reason"))
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

	runtimeBuilder := NewRuntimeBuilder(reconRepo, logger.NewLogger(debugLogging))

	//use a channel because callbacks are invoked from multiple goroutines
	callbackData := make(chan *reconciler.CallbackMessage, 10)
	localRunner := runtimeBuilder.RunLocal(func(component string, msg *reconciler.CallbackMessage) {
		callbackData <- msg
	}).WithWorkerPoolMaxRetries(1).WithSchedulerConfig(&SchedulerConfig{})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	reconResult, err := localRunner.Run(ctx, clusterState)
	require.NoError(t, err)

	//Collect received callbacks
	var receivedUpdates []*reconciler.CallbackMessage
	close(callbackData)
	for msg := range callbackData {
		receivedUpdates = append(receivedUpdates, msg)
	}

	return reconResult, receivedUpdates
}

func countStatus(status reconciler.Status, receivedUpdates []*reconciler.CallbackMessage) int {
	count := 0
	for _, recv := range receivedUpdates {
		if recv.Status == status {
			count++
		}
	}
	return count
}
