package eventing

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func TestRunner(t *testing.T) {
	t.Run("Should register Eventing reconciler", func(t *testing.T) {
		reconciler, err := service.GetReconciler(reconcilerName)
		require.NoError(t, err)
		require.NotNil(t, reconciler)
	})
}
