package worker

import (
	"context"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/invoker"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

type testInvoker struct {
	params []*invoker.Params
}

func (i *testInvoker) Invoke(_ context.Context, params *invoker.Params) error {
	i.params = append(i.params, params)
	return nil
}

func TestWorkerPool(t *testing.T) {
	test.IntegrationTest(t) //required because a valid Kubeconfig is required to create test cluster entry

	//create cluster inventory
	inventory, err := cluster.NewInventory(db.NewTestConnection(t), true, &cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	//add cluster to inventory
	clusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
		Kubeconfig: test.ReadKubeconfig(t),
		KymaConfig: keb.KymaConfig{
			Administrators: nil,
			Components: []keb.Component{
				{
					Component: "TestComp1",
				},
			},
			Profile: "",
			Version: "1.2.3",
		},
		Metadata:     keb.Metadata{},
		RuntimeID:    "testCluster",
		RuntimeInput: keb.RuntimeInput{},
	})
	require.NoError(t, err)

	//cleanup created cluster
	defer func() {
		require.NoError(t, inventory.Delete(clusterState.Cluster.RuntimeID))
	}()

	//create reconciliation for cluster
	reconRepo := reconciliation.NewInMemoryReconciliationRepository()
	reconEntity, err := reconRepo.CreateReconciliation(clusterState, nil)
	require.NoError(t, err)
	opsProcessable, err := reconRepo.GetProcessableOperations(0)
	require.Len(t, opsProcessable, 2)
	require.NoError(t, err)

	//create test invoker to be able to verify invoker calls
	testInvoker := &testInvoker{}

	//start worker pool
	workerPool, err := NewWorkerPool(&InventoryRetriever{inventory}, reconRepo, testInvoker, nil, logger.NewLogger(true))
	require.NoError(t, err)

	//create time limited context
	ctx, cancelFct := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelFct()

	//ensure worker pool stops when context gets closed
	startTime := time.Now()
	require.NoError(t, workerPool.Run(ctx))
	require.WithinDuration(t, startTime, time.Now(), 3*time.Second) //ensure workerPool is considering ctx

	//verify that invoker was properly called
	require.Len(t, testInvoker.params, 2)
	require.Equal(t, clusterState, testInvoker.params[0].ClusterState)
	require.Equal(t, clusterState, testInvoker.params[1].ClusterState)

	//CRDs and Cleanup are called in random order.
	var crdsParams *invoker.Params
	var cleanupParams *invoker.Params
	if testInvoker.params[0].ComponentToReconcile.Component == model.CRDComponent {
		crdsParams = testInvoker.params[0]
		cleanupParams = testInvoker.params[1]
	} else if testInvoker.params[0].ComponentToReconcile.Component == model.CleanupComponent {
		crdsParams = testInvoker.params[1]
		cleanupParams = testInvoker.params[0]
	} else {
		t.Fatalf("Unexpected component: %s", testInvoker.params[0].ComponentToReconcile.Component)
	}
	require.Equal(t, reconEntity.SchedulingID, crdsParams.SchedulingID)
	require.Equal(t, reconEntity.SchedulingID, cleanupParams.SchedulingID)

	requireOpsProcessableExists(t, opsProcessable, crdsParams.CorrelationID)
	requireOpsProcessableExists(t, opsProcessable, cleanupParams.CorrelationID)
}

func requireOpsProcessableExists(t *testing.T, opsProcessable []*model.OperationEntity, correlationID string) {
	for i := range opsProcessable {
		if opsProcessable[i].CorrelationID == correlationID {
			return
		}
	}
	t.Fatalf("Could not find correlationID: %s in opsProcessable list: %v", correlationID, opsProcessable)
}
