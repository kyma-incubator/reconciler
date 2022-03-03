package service

import (
	"context"
	"testing"
	"time"

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
			conn: db.NewTestConnection(t),
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
			logger: logger.NewLogger(true),
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

		reconRepo := reconciliation.MockRepository{
			RemoveReconciliationResult: nil,
			GetReconciliationsResult: []*model.ReconciliationEntity{
				{
					SchedulingID: "test-id-0",
					Created:      start,
					Status:       model.ClusterStatusReady,
				},
				{
					SchedulingID: "test-id-1",
					Created:      start.Add((-24*1 - 1) * time.Hour),
					Status:       model.ClusterStatusReconcileError,
				},
				{
					SchedulingID: "test-id-2",
					Created:      start.Add((-24*2 - 1) * time.Hour),
					Status:       model.ClusterStatusReady,
				},
				{
					SchedulingID: "test-id-3",
					Created:      start.Add((-24*3 - 1) * time.Hour),
					Status:       model.ClusterStatusReconcileError,
				},
				{
					SchedulingID: "test-id-4",
					Created:      start.Add((-24*4 - 1) * time.Hour),
					Status:       model.ClusterStatusReady,
				},
				{
					SchedulingID: "test-id-5",
					Created:      start.Add((-24*5 - 1) * time.Hour),
					Status:       model.ClusterStatusReconcileError,
				},
				{
					SchedulingID: "test-id-6",
					Created:      start.Add((-24*6 - 1) * time.Hour),
					Status:       model.ClusterStatusReady,
				},
				{
					SchedulingID: "test-id-7",
					Created:      start.Add((-24*7 - 1) * time.Hour),
					Status:       model.ClusterStatusReconcileError,
				},
				{
					SchedulingID: "test-id-8",
					Created:      start.Add((-24*8 - 1) * time.Hour),
					Status:       model.ClusterStatusReady,
				},
				{
					SchedulingID: "test-id-9",
					Created:      start.Add((-24*9 - 1) * time.Hour),
					Status:       model.ClusterStatusReconcileError,
				},
				{
					SchedulingID: "test-id-10",
					Created:      start.Add((-24*10 - 1) * time.Hour),
					Status:       model.ClusterStatusReady,
				},
			},
		}

		err := cleaner.Run(ctx, &ClusterStatusTransition{
			conn:      db.NewTestConnection(t),
			reconRepo: &reconRepo,
			logger:    logger.NewLogger(true),
		}, &CleanerConfig{
			KeepLatestEntitiesCount: 5,
			MaxEntitiesAgeDays:      3,
			CleanerInterval:         5 * time.Second,
		})

		require.NoError(t, err)

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

func Test_diffDays(t *testing.T) {
	type test struct {
		time1    string
		time2    string
		expected int
	}

	cases := []test{
		{time1: "2022-01-19T05:22:41Z", time2: "2022-01-19T05:21:41Z", expected: 0},   //time1 > time2
		{time1: "2022-01-19T05:21:41Z", time2: "2022-01-19T05:21:41Z", expected: 0},   //time1 == time2
		{time1: "2022-01-19T05:20:40Z", time2: "2022-01-19T05:21:41Z", expected: 0},   //time1 == time2 - 1 minute
		{time1: "2022-01-18T05:21:41Z", time2: "2022-01-19T05:21:41Z", expected: 1},   //time1 == time2 - 1 day
		{time1: "2022-01-16T05:21:41Z", time2: "2022-01-19T05:21:41Z", expected: 3},   //time1 == time2 - 3 days
		{time1: "2021-01-19T05:21:41Z", time2: "2022-01-19T05:21:41Z", expected: 365}, //time1 == time2 - 1 year
	}

	for _, tc := range cases {
		time1, err := time.Parse(time.RFC3339, tc.time1)
		require.NoError(t, err)
		time2, err := time.Parse(time.RFC3339, tc.time2)
		require.NoError(t, err)
		require.Equal(t, tc.expected, diffDays(time1, time2), "For time %s and %s", tc.time1, tc.time2)
	}
}
