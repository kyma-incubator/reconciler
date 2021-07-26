package compreconciler

import (
	"context"
	"fmt"
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

func (cb *testCallbackHandler) Callback(status Status) error {
	return os.Setenv("_testCallbackHandlerStatuses", fmt.Sprintf("%s,%s", os.Getenv("_testCallbackHandlerStatuses"), status))
}

func (cb *testCallbackHandler) Statuses() []Status {
	statuses := strings.Split(os.Getenv("_testCallbackHandlerStatuses"), ",")
	var result []Status
	for _, status := range statuses {
		result = append(result, Status(status))
	}
	return result
}

func (cb *testCallbackHandler) LatestStatus() Status {
	statuses := strings.Split(os.Getenv("_testCallbackHandlerStatuses"), ",")
	return Status(statuses[len(statuses)-1])
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

		statusUpdater, err := newStatusUpdater(ctx, callbackHdlr, true, StatusUpdaterConfig{
			Interval:   1 * time.Second,
			MaxRetries: 1,
			RetryDelay: 1 * time.Second,
		})
		require.NoError(t, err)
		require.Equal(t, statusUpdater.CurrentStatus(), NotStarted)

		require.NoError(t, statusUpdater.Running())
		require.Equal(t, statusUpdater.CurrentStatus(), Running)
		time.Sleep(2 * time.Second)

		require.NoError(t, statusUpdater.Failed())
		require.Equal(t, statusUpdater.CurrentStatus(), Failed)
		time.Sleep(2 * time.Second)

		require.NoError(t, statusUpdater.Success())
		require.Equal(t, statusUpdater.CurrentStatus(), Success)
		time.Sleep(2 * time.Second)

		//check fired status updates
		require.GreaterOrEqual(t, len(callbackHdlr.Statuses()), 4) //anything > 3 is sufficient to ensure the statusUpdaters works
		require.Equal(t, callbackHdlr.LatestStatus(), Success)
	})

	t.Run("Test status updater with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		callbackHdlr := &testCallbackHandler{}

		statusUpdater, err := newStatusUpdater(ctx, callbackHdlr, true, StatusUpdaterConfig{
			Interval:   1 * time.Second,
			MaxRetries: 1,
			RetryDelay: 1 * time.Second,
		})
		require.NoError(t, err)
		require.Equal(t, statusUpdater.CurrentStatus(), NotStarted)

		require.NoError(t, statusUpdater.Running())
		require.Equal(t, statusUpdater.CurrentStatus(), Running)
		time.Sleep(4 * time.Second) //wait longer than timeout to simulate expired context

		//check fired status updates
		require.GreaterOrEqual(t, len(callbackHdlr.Statuses()), 2) //anything > 1 is sufficient to ensure the statusUpdaters worked

		require.Error(t, statusUpdater.Failed()) //status changes have to fail after status-updater was interrupted
	})
}
