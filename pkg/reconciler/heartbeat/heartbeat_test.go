package heartbeat

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/stretchr/testify/require"
)

//testCallbackHandler is tracking fired status-updates in an env-var (allows a stateless callback implementation)
//This implementation CAN NOT RUN IN PARALLEL!
type testCallbackHandler struct {
}

func newTestCallbackHandler(t *testing.T) *testCallbackHandler {
	require.NoError(t, os.Unsetenv("_testCallbackHandlerStatuses"))
	return &testCallbackHandler{}
}

func (cb *testCallbackHandler) Callback(msg *reconciler.CallbackMessage) error {
	statusList := os.Getenv("_testCallbackHandlerStatuses")
	if statusList == "" {
		statusList = string(msg.Status)
	} else {
		statusList = fmt.Sprintf("%s,%s", statusList, msg.Status)
	}
	return os.Setenv("_testCallbackHandlerStatuses", statusList)
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

func TestHeartbeatSender(t *testing.T) { //DO NOT RUN THIS TEST CASES IN PARALLEL!
	test.IntegrationTest(t)

	t.Parallel()

	logger := log.NewOptionalLogger(true)
	t.Run("Test heartbeat sender without timeout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		callbackHdlr := newTestCallbackHandler(t)

		heartbeatSender, err := NewHeartbeatSender(ctx, callbackHdlr, logger, Config{
			Interval: 500 * time.Millisecond,
			Timeout:  10 * time.Second,
		})
		require.NoError(t, err)
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.NotStarted)

		require.NoError(t, heartbeatSender.Running())
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.Running)
		time.Sleep(2 * time.Second)

		require.NoError(t, heartbeatSender.Success())
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.Success)
		time.Sleep(2 * time.Second)

		//check fired status updates
		require.GreaterOrEqual(t, len(callbackHdlr.Statuses()), 4) //anything >= 4 is sufficient to ensure the heartbeatSenders works
		require.Equal(t, callbackHdlr.LatestStatus(), reconciler.Success)
	})

	t.Run("Test heartbeat sender with context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		callbackHdlr := newTestCallbackHandler(t)

		heartbeatSender, err := NewHeartbeatSender(ctx, callbackHdlr, logger, Config{
			Interval: 1 * time.Second,
			Timeout:  10 * time.Second,
		})
		require.NoError(t, err)
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.NotStarted)

		require.NoError(t, heartbeatSender.Running())
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.Running)

		time.Sleep(3 * time.Second) //wait longer than timeout to simulate expired context

		require.True(t, heartbeatSender.isContextClosed()) //verify that status-updater received timeout

		//check fired status updates
		require.GreaterOrEqual(t, len(callbackHdlr.Statuses()), 2) //anything > 1 is sufficient to ensure the heartbeatSenders worked
	})

	t.Run("Test heartbeat sender with heartbeat sender timeout", func(t *testing.T) {
		callbackHdlr := newTestCallbackHandler(t)

		heartbeatSender, err := NewHeartbeatSender(context.Background(), callbackHdlr, logger, Config{
			Interval: 500 * time.Millisecond,
			Timeout:  1 * time.Second,
		})
		require.NoError(t, err)
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.NotStarted)

		require.NoError(t, heartbeatSender.Running())
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.Running)

		time.Sleep(2 * time.Second) //wait longer than status update timeout to timeout

		//check fired status updates: anything 1 >= x <= 3 is sufficient to ensure the heartbeatSenders worked
		require.GreaterOrEqual(t, len(callbackHdlr.Statuses()), 1)
		require.LessOrEqual(t, len(callbackHdlr.Statuses()), 3)
	})

}
