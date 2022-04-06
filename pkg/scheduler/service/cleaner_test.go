package service

import (
	"context"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/stretchr/testify/require"
)

func Test_cleaner_Run(t *testing.T) {

	t.Run("Test run with old logic", func(t *testing.T) {
		cleaner := newCleaner(logger.NewLogger(true))

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		start := time.Now()

		err := cleaner.Run(ctx, &ClusterStatusTransition{
			conn: &db.MockConnection{},
			reconRepo: &reconciliation.MockRepository{
				RemoveReconciliationResult: nil,
				GetReconciliationsResult: []*model.ReconciliationEntity{
					{
						SchedulingID: "test-id-1",
					},
					{
						SchedulingID: "test-id-2",
					},
				},
			},
			inventory: &cluster.MockInventory{},
			logger:    logger.NewLogger(true),
		}, &CleanerConfig{
			PurgeEntitiesOlderThan: 2 * time.Second,
			CleanerInterval:        5 * time.Second,
		})
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond) //give it some time to shutdown

		require.WithinDuration(t, start, time.Now(), 2*time.Second)
	})

	t.Run("Test run with new logic", func(t *testing.T) {
		cleaner := newCleaner(logger.NewLogger(true))

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		start := time.Now()

		mockCluster := model.ClusterEntity{
			RuntimeID: "test-cluster",
		}
		mockClusterState := cluster.State{
			Cluster: &mockCluster,
		}
		mockInventory := cluster.MockInventory{
			GetAllResult: []*cluster.State{&mockClusterState},
		}

		reconciliations := []*model.ReconciliationEntity{
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-0",
				Created:      start,
				Status:       model.ClusterStatusReady,
			},
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-1",
				Created:      start.Add((-1 * 24) * time.Hour),
				Status:       model.ClusterStatusReconcileErrorRetryable,
			},
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-2",
				Created:      start.Add((-2 * 24) * time.Hour),
				Status:       model.ClusterStatusReady,
			},
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-3",
				Created:      start.Add((-3 * 24) * time.Hour),
				Status:       model.ClusterStatusReconcileErrorRetryable,
			},
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-4",
				Created:      start.Add((-4 * 24) * time.Hour),
				Status:       model.ClusterStatusReady,
			},
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-5",
				Created:      start.Add((-5 * 24) * time.Hour),
				Status:       model.ClusterStatusReconcileErrorRetryable,
			},
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-6",
				Created:      start.Add((-6 * 24) * time.Hour),
				Status:       model.ClusterStatusReady,
			},
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-7",
				Created:      start.Add((-7 * 24) * time.Hour),
				Status:       model.ClusterStatusReconcileErrorRetryable,
			},
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-8",
				Created:      start.Add((-8 * 24) * time.Hour),
				Status:       model.ClusterStatusReady,
			},
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-9",
				Created:      start.Add((-9 * 24) * time.Hour),
				Status:       model.ClusterStatusReconcileErrorRetryable,
			},
			{
				RuntimeID:    "test-cluster",
				SchedulingID: "test-id-10",
				Created:      start.Add((-10 * 24) * time.Hour),
				Status:       model.ClusterStatusReady,
			},
		}

		updateModel := func(repo *reconciliation.MockRepository) {
			if repo.GetReconciliationsCount == 1 {
				repo.GetReconciliationsResult = reconciliations
			} else if repo.GetReconciliationsCount == 2 {
				repo.GetReconciliationsResult = reconciliations[0:6]
			}
		}

		reconRepo := reconciliation.MockRepository{
			RemoveReconciliationResult: nil,
			GetReconciliationsResult:   []*model.ReconciliationEntity{reconciliations[0]}, //data for the first call for a most recent reconciliation
			OnGetReconciliations:       updateModel,
		}

		err := cleaner.Run(ctx, &ClusterStatusTransition{
			conn:      &db.MockConnection{},
			reconRepo: &reconRepo,
			inventory: &mockInventory,
			logger:    logger.NewLogger(true),
		}, &CleanerConfig{
			KeepLatestEntitiesCount: 4,
			MaxEntitiesAgeDays:      6,
			CleanerInterval:         5 * time.Second,
		})

		require.NoError(t, err)
		require.Equal(t, 2, reconRepo.GetReconciliationsCount)
		recordedIDs := reconRepo.RemoveReconciliationRecording

		//First batch of removals comes from "deleteRecordsByAge"
		require.Equal(t, "test-id-7", recordedIDs[0])
		require.Equal(t, "test-id-8", recordedIDs[1])
		require.Equal(t, "test-id-9", recordedIDs[2])
		require.Equal(t, "test-id-10", recordedIDs[3])

		//Second batch of removals comes from "deleteRecordsByAge"
		require.Equal(t, "test-id-4", recordedIDs[4])
		require.Equal(t, "test-id-6", recordedIDs[5])

		time.Sleep(500 * time.Millisecond) //give it some time to shutdown

		require.WithinDuration(t, start, time.Now(), 2*time.Second)
	})
}

func Test_beginningOfTheDay(t *testing.T) {
	type test struct {
		time     string
		expected string
	}

	exp := "2022-01-19T00:00:00Z"
	cases := []test{
		{time: "2022-01-19T05:21:41Z", expected: exp},
		{time: "2022-01-19T23:59:59.999Z", expected: exp},
		{time: "2022-01-19T00:00:00.000Z", expected: exp},
	}

	for _, tc := range cases {
		given, err := time.Parse(time.RFC3339, tc.time)
		require.NoError(t, err)
		actual := beginningOfTheDay(given)
		require.Equal(t, tc.expected, actual.Format(time.RFC3339))
	}
}
