package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testCase struct {
	operations      []*model.OperationEntity
	expectedResults map[string]model.Status
	expectedOrphans []string //contains correlation IDs
}

func TestBookkeeper(t *testing.T) {
	dbConn := db.NewTestConnection(t) //share one db-connection between inventory and recon-repo (required for tx)

	//prepare inventory
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)
	clusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
		Kubeconfig: "123",
		KymaConfig: keb.KymaConfig{
			Components: nil,
			Profile:    "",
			Version:    "1.2.3",
		},
		RuntimeID: "testCluster",
	})
	require.NoError(t, err)

	//trigger reconciliation for cluster
	reconRepo, err := reconciliation.NewPersistedReconciliationRepository(dbConn, true)
	require.NoError(t, err)
	reconEntity, err := reconRepo.CreateReconciliation(clusterState, nil)
	require.NoError(t, err)
	require.NotEmpty(t, reconEntity.Lock)
	require.True(t, reconEntity.IsReconciling())

	//mark all operations to be finished
	opEntities, err := reconRepo.GetOperations(reconEntity.SchedulingID)
	require.NoError(t, err)
	for _, opEntity := range opEntities {
		err := reconRepo.UpdateOperationState(opEntity.SchedulingID, opEntity.CorrelationID, model.OperationStateDone)
		require.NoError(t, err)
	}

	//initialize bookkeeper
	bk := newBookkeeper(
		newClusterStatusTransition(dbConn, inventory, reconRepo, logger.NewLogger(true)),
		&BookkeeperConfig{
			OperationsWatchInterval: 1 * time.Second,
			OrphanOperationTimeout:  2 * time.Second,
		},
		logger.NewLogger(true))

	//run bookkeeper
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //stop bookkeeper after 2 sec
	defer cancel()

	start := time.Now()
	require.NoError(t, bk.Run(ctx))
	require.WithinDuration(t, time.Now(), start, 5500*time.Millisecond) //verify bookkeeper stops when ctx gets closed

	//verify bookkeeper results
	reconEntityUpdated, err := reconRepo.GetReconciliation(reconEntity.SchedulingID)
	require.NoError(t, err)
	require.Empty(t, reconEntityUpdated.Lock)
	require.False(t, reconEntityUpdated.IsReconciling())
}

func TestBookkeeper_processClusterStateAndOrphans(t *testing.T) {
	bk := newBookkeeper(nil, &BookkeeperConfig{
		OrphanOperationTimeout: 2 * time.Second,
	}, logger.NewLogger(true))

	testCases := []*testCase{
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateNew,
					Updated:       time.Now().Add(-1999 * time.Millisecond),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.2",
					State:         model.OperationStateError,
					Updated:       time.Now().Add(-2000 * time.Millisecond),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.3",
					State:         model.OperationStateInProgress,
					Updated:       time.Now().Add(-2001 * time.Millisecond),
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusError,
			},
			expectedOrphans: []string{"1.3"},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateFailed,
					Updated:       time.Now().Add(-3 * time.Second),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.2",
					State:         model.OperationStateNew,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.3",
					State:         model.OperationStateInProgress,
					Updated:       time.Now(),
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusReconciling,
			},
			expectedOrphans: []string{"1.1"},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateDone,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.2",
					State:         model.OperationStateNew,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.3",
					State:         model.OperationStateInProgress,
					Updated:       time.Now(),
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusReconciling,
			},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateDone,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "2",
					CorrelationID: "2.1",
					State:         model.OperationStateNew,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "2",
					CorrelationID: "2.2",
					State:         model.OperationStateInProgress,
					Updated:       time.Now(),
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusReady,
				"2": model.ClusterStatusReconciling,
			},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateDone,
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.2",
					State:         model.OperationStateDone,
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusReady,
			},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateError,
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.2",
					State:         model.OperationStateDone,
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusError,
			},
		},
	}
	for _, testCase := range testCases {
		reconResults, err := bk.processReconciliations(testCase.operations)
		require.NoError(t, err)
		require.Equal(t, len(testCase.expectedResults), len(reconResults))

		//check calculated cluster results
		for schedulingID, expectedStatus := range testCase.expectedResults {
			reconResult, ok := reconResults[schedulingID]
			require.True(t, ok)
			require.Equal(t, expectedStatus, reconResult.GetResult())
		}

		//check detected orphans
		allDetectedOrphans := make(map[string]*model.OperationEntity)
		for _, reconResult := range reconResults {
			detectedOrphans := reconResult.GetOrphans()
			for _, detectedOrphan := range detectedOrphans {
				allDetectedOrphans[detectedOrphan.CorrelationID] = detectedOrphan
			}
		}
		for _, correlationID := range testCase.expectedOrphans {
			_, ok := allDetectedOrphans[correlationID]
			require.True(t, ok)
		}
	}
}
