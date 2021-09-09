package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWorkerPool(t *testing.T) {
	t.Run("Filter missing component dependencies", func(t *testing.T) {
		recon, err := NewComponentReconciler("unittest")
		require.NoError(t, err)
		require.NoError(t, recon.Debug())
		recon.WithDependencies("a", "b")

		wp := WorkerPool{
			reconciler: recon,
			workerPool: nil,
			ctx:        nil,
		}

		require.ElementsMatch(t,
			[]string{"a", "b"},
			wp.Reconcilable(&reconciler.Reconciliation{
				ComponentsReady: []string{"x", "y", "z"},
			}).Missing)
		require.ElementsMatch(t,
			[]string{"b"},
			wp.Reconcilable(&reconciler.Reconciliation{
				ComponentsReady: []string{"a", "y", "z"},
			}).Missing)
		require.ElementsMatch(t,
			[]string{},
			wp.Reconcilable(&reconciler.Reconciliation{
				ComponentsReady: []string{"a", "b", "z"},
			}).Missing)
	})
}
