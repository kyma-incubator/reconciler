package callback

import (
	"fmt"
	"testing"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestRemoteCallbackHandler(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	logger := log.NewOptionalLogger(true)

	t.Run("Test successful remote status update", func(t *testing.T) {
		rcb, err := NewRemoteCallbackHandler("https://httpbin.org/status/200", logger)
		require.NoError(t, err)
		require.NoError(t, rcb.Callback(reconciler.Running))
	})

	t.Run("Test failed remote status update", func(t *testing.T) {
		rcb, err := NewRemoteCallbackHandler("https://httpbin.org/status/400", logger)
		require.NoError(t, err)
		require.Error(t, rcb.Callback(reconciler.Running))
	})
}

func TestLocalCallbackHandler(t *testing.T) {
	logger := log.NewOptionalLogger(true)

	t.Run("Test successful local status update", func(t *testing.T) {
		var localFctCalled bool
		rcb, err := NewLocalCallbackHandler(func(status reconciler.Status) error {
			localFctCalled = true
			return nil
		}, logger)
		require.NoError(t, err)
		require.NoError(t, rcb.Callback(reconciler.Running))
		require.True(t, localFctCalled)
	})

	t.Run("Test failed local status update", func(t *testing.T) {
		rcb, err := NewLocalCallbackHandler(func(status reconciler.Status) error {
			return fmt.Errorf("I failed")
		}, logger)
		require.NoError(t, err)
		require.Error(t, rcb.Callback(reconciler.Running))
	})
}
