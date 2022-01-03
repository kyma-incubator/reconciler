package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/stretchr/testify/require"
)

func Test_cleaner_Run(t *testing.T) {
	t.Run("Test run", func(t *testing.T) {
		cleaner := newCleaner(logger.NewLogger(true))

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		start := time.Now()

		err := cleaner.Run(ctx, &ClusterStatusTransition{
			conn: test.NewTestConnection(t),
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
}
