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
	test.IntegrationTest(t)

	logger := log.NewLogger(true)

	t.Run("Test successful remote status update", func(t *testing.T) {
		rcb, err := NewRemoteCallbackHandler("https://httpbin.org/status/200", logger)
		require.NoError(t, err)
		require.NoError(t, rcb.Callback(&reconciler.CallbackMessage{
			Status: reconciler.Running,
			Error:  nil,
		}))
	})

	t.Run("Test failed remote status update", func(t *testing.T) {
		rcb, err := NewRemoteCallbackHandler("https://httpbin.org/status/400", logger)
		require.NoError(t, err)
		require.Error(t, rcb.Callback(&reconciler.CallbackMessage{
			Status: reconciler.Running,
			Error:  nil,
		}))
	})
}

func TestLocalCallbackHandler(t *testing.T) {
	logger := log.NewLogger(true)

	t.Run("Test successful local status update", func(t *testing.T) {
		var localFuncCalled bool
		rcb, err := NewLocalCallbackHandler(func(msg *reconciler.CallbackMessage) error {
			localFuncCalled = true
			return nil
		}, logger)
		require.NoError(t, err)
		require.NoError(t, rcb.Callback(&reconciler.CallbackMessage{
			Status: reconciler.Running,
			Error:  nil,
		}))
		require.True(t, localFuncCalled)
	})

	t.Run("Test failed local status update", func(t *testing.T) {
		rcb, err := NewLocalCallbackHandler(func(msg *reconciler.CallbackMessage) error {
			return fmt.Errorf("I failed")
		}, logger)
		require.NoError(t, err)
		require.Error(t, rcb.Callback(&reconciler.CallbackMessage{
			Status: reconciler.Running,
			Error:  nil,
		}))
	})
}
