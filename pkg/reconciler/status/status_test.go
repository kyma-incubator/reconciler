package status

import (
	"context"
	"fmt"
	e "github.com/kyma-incubator/reconciler/pkg/error"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
	"time"
)

//testCallbackHandler is tracking fired status-updates in an env-var (allows a stateless callback implementation)
type testCallbackHandler struct {
}

func (cb *testCallbackHandler) Callback(status reconciler.Status) error {
	return os.Setenv("_testCallbackHandlerStatuses", fmt.Sprintf("%s,%s", os.Getenv("_testCallbackHandlerStatuses"), status))
}

func (cb *testCallbackHandler) Statuses() []reconciler.Status {
	statuses := strings.Split(os.Getenv("_testCallbackHandlerStatuses"), ",")
	var result []reconciler.Status
	for _, status := range statuses {
		result = append(result, reconciler.Status(status))
	}
	return result
}

func (cb *testCallbackHandler) LatestStatus() reconciler.Status {
	statuses := strings.Split(os.Getenv("_testCallbackHandlerStatuses"), ",")
	return reconciler.Status(statuses[len(statuses)-1])
}

func TestStatusUpdater(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	t.Parallel()

	t.Run("Test status updater without timeout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		callbackHdlr := &testCallbackHandler{}

		statusUpdater, err := NewStatusUpdater(ctx, callbackHdlr, true, Config{
			Interval:   1 * time.Second,
			MaxRetries: 1,
			RetryDelay: 1 * time.Second,
		})
		require.NoError(t, err)
		require.Equal(t, statusUpdater.CurrentStatus(), reconciler.NotStarted)

		require.NoError(t, statusUpdater.Running())
		require.Equal(t, statusUpdater.CurrentStatus(), reconciler.Running)
		time.Sleep(2 * time.Second)

		require.NoError(t, statusUpdater.Failed())
		require.Equal(t, statusUpdater.CurrentStatus(), reconciler.Failed)
		time.Sleep(2 * time.Second)

		require.NoError(t, statusUpdater.Success())
		require.Equal(t, statusUpdater.CurrentStatus(), reconciler.Success)
		time.Sleep(2 * time.Second)

		//check fired status updates
		require.GreaterOrEqual(t, len(callbackHdlr.Statuses()), 4) //anything > 3 is sufficient to ensure the statusUpdaters works
		require.Equal(t, callbackHdlr.LatestStatus(), reconciler.Success)
	})

	t.Run("Test status updater with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		callbackHdlr := &testCallbackHandler{}

		statusUpdater, err := NewStatusUpdater(ctx, callbackHdlr, true, Config{
			Interval:   1 * time.Second,
			MaxRetries: 1,
			RetryDelay: 1 * time.Second,
		})
		require.NoError(t, err)
		require.Equal(t, statusUpdater.CurrentStatus(), reconciler.NotStarted)

		require.NoError(t, statusUpdater.Running())
		require.Equal(t, statusUpdater.CurrentStatus(), reconciler.Running)
		time.Sleep(4 * time.Second) //wait longer than timeout to simulate expired context

		//check fired status updates
		require.GreaterOrEqual(t, len(callbackHdlr.Statuses()), 2) //anything > 1 is sufficient to ensure the statusUpdaters worked

		err = statusUpdater.Failed()
		require.Error(t, err)
		require.IsType(t, &e.ContextClosedError{}, err) //status changes have to fail after status-updater was interrupted
	})

}
