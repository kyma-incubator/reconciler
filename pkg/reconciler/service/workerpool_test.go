package service

import (
	"context"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestWorkerPool(t *testing.T) {
	t.Run("Filter missing component dependencies", func(t *testing.T) {
		recon, err := NewComponentReconciler("unittest")
		require.NoError(t, err)

		//configure reconciler
		recon.Debug().WithDependencies("a", "b")

		ctx, cancel := context.WithCancel(context.TODO())

		wp, err := newWorkerPoolBuilder(&dependencyChecker{}, newRunnerFct()).
			WithPoolSize(5).
			WithDebug(true).
			Build(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, wp.antsPool)
		require.False(t, wp.antsPool.IsClosed())
		require.NotEmpty(t, wp.logger)
		require.True(t, wp.debug)
		require.NotEmpty(t, wp.newRunnerFct)
		require.IsType(t, &dependencyChecker{}, wp.depChecker)

		//shutdown pool
		cancel()
		time.Sleep(500 * time.Millisecond) //give ants-pool some time to shutdown
		require.True(t, wp.antsPool.IsClosed())
	})
}

func newRunnerFct() func(context.Context, *reconciler.Task, callback.Handler, *zap.SugaredLogger) func() error {
	return func(ctx context.Context, reconciliation *reconciler.Task, handler callback.Handler, logger *zap.SugaredLogger) func() error {
		return func() error {
			return nil
		}
	}
}
