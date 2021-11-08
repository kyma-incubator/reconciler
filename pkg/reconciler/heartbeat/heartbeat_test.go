package heartbeat

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/pkg/errors"
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
	t *testing.T
}

func newTestCallbackHandler(t *testing.T) *testCallbackHandler {
	require.NoError(t, os.Unsetenv("_testCallbackHandlerStatuses"))
	return &testCallbackHandler{t}
}

func (cb *testCallbackHandler) Callback(msg *reconciler.CallbackMessage) error {
	cb.t.Logf("Sending callback: %s", msg)
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

	logger := log.NewLogger(true)

	t.Run("Test heartbeat sender without timeout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		callbackHdlr := newTestCallbackHandler(t)

		heartbeatSender, err := NewHeartbeatSender(ctx, callbackHdlr, logger, Config{
			Interval: 500 * time.Millisecond,
			Timeout:  10 * time.Second,
		})
		require.NoError(t, err)
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.StatusNotstarted)

		require.NoError(t, heartbeatSender.Running())
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.StatusRunning)
		time.Sleep(2 * time.Second)

		require.NoError(t, heartbeatSender.Failed(errors.New("I'm currently failing")))
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.StatusFailed)
		time.Sleep(2 * time.Second)

		require.NoError(t, heartbeatSender.Success())
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.StatusSuccess)
		time.Sleep(2 * time.Second)

		//check fired status updates
		require.GreaterOrEqual(t, len(callbackHdlr.Statuses()), 4) //anything >= 4 is sufficient to ensure the heartbeatSenders works
		require.Equal(t, callbackHdlr.LatestStatus(), reconciler.StatusSuccess)
	})

	t.Run("Test heartbeat sender with context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		callbackHdlr := newTestCallbackHandler(t)

		heartbeatSender, err := NewHeartbeatSender(ctx, callbackHdlr, logger, Config{
			Interval: 500 * time.Millisecond,
			Timeout:  10 * time.Second,
		})
		require.NoError(t, err)
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.StatusNotstarted)

		require.NoError(t, heartbeatSender.Running())
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.StatusRunning)

		time.Sleep(3 * time.Second) //wait longer than timeout to simulate expired context

		require.True(t, heartbeatSender.isContextClosed()) //verify that status-updater received timeout

		//check fired status updates
		statuses := callbackHdlr.Statuses()
		require.GreaterOrEqual(t, len(statuses), 2) //anything >= 2 is sufficient to ensure the heartbeatSenders worked
		require.Equal(t, statuses[len(statuses)-1], reconciler.StatusError)
	})

	t.Run("Test heartbeat sender with context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		callbackHdlr := newTestCallbackHandler(t)

		heartbeatSender, err := NewHeartbeatSender(ctx, callbackHdlr, logger, Config{
			Interval: 500 * time.Millisecond,
			Timeout:  10 * time.Second,
		})
		require.NoError(t, err)
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.StatusNotstarted)

		require.NoError(t, heartbeatSender.Running())
		require.Equal(t, heartbeatSender.CurrentStatus(), reconciler.StatusRunning)

		time.Sleep(3 * time.Second) //wait longer than timeout to simulate expired context
		cancel()
		time.Sleep(250 * time.Millisecond) //give heartbeat some time to close its context

		require.True(t, heartbeatSender.isContextClosed()) //verify that status-updater received timeout

		//check fired status updates
		statuses := callbackHdlr.Statuses()
		require.GreaterOrEqual(t, len(statuses), 2) //anything >= 2 is sufficient to ensure the heartbeatSenders worked
		require.Equal(t, statuses[len(statuses)-1], reconciler.StatusFailed)
	})

}
