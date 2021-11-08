package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type DummyAction struct {
	receivedVersion string
	receivedProfile string
	receivedConfig  map[string]interface{}
}

func (da *DummyAction) Run(helper *ActionContext) error {
	if helper.KubeClient != nil {
		return fmt.Errorf("kubeClient is not expected in this test case")
	}
	da.receivedVersion = helper.Task.Version
	da.receivedProfile = helper.Task.Profile
	da.receivedConfig = helper.Task.Configuration
	return nil
}

func TestReconciler(t *testing.T) {

	t.Run("Verify fluent configuration interface", func(t *testing.T) {
		recon, err := NewComponentReconciler("unittest")
		require.NoError(t, err)

		recon.Debug()
		require.NotEmpty(t, recon.logger)
		require.True(t, recon.debug)

		recon.WithWorkspace("./test")
		require.Equal(t, "./test", recon.workspace)

		//verify retry config
		recon.WithRetry(111, 222*time.Second)
		require.Equal(t, 111, recon.maxRetries)
		require.Equal(t, 222*time.Second, recon.retryDelay)

		//verify dependencies
		recon.WithDependencies("a", "b", "c")
		require.Equal(t, []string{"a", "b", "c"}, recon.dependencies)

		//verify pre, post and install-action
		preAct := &DummyAction{
			"123",
			"",
			nil,
		}
		instAct := &DummyAction{
			"123",
			"",
			nil,
		}
		postAct := &DummyAction{
			"123",
			"",
			nil,
		}
		recon.WithPreReconcileAction(preAct).
			WithReconcileAction(instAct).
			WithPostReconcileAction(postAct)
		require.Equal(t, preAct, recon.preReconcileAction)
		require.Equal(t, instAct, recon.reconcileAction)
		require.Equal(t, postAct, recon.postReconcileAction)

		recon.WithHeartbeatSenderConfig(333*time.Second, 4455*time.Second)
		require.Equal(t, 333*time.Second, recon.heartbeatSenderConfig.interval)
		require.Equal(t, 4455*time.Second, recon.heartbeatSenderConfig.timeout)

		recon.WithProgressTrackerConfig(666*time.Second, 777*time.Second)
		require.Equal(t, 666*time.Second, recon.progressTrackerConfig.interval)
		require.Equal(t, 777*time.Second, recon.progressTrackerConfig.timeout)

		recon.WithWorkers(888, 999*time.Second)
		require.Equal(t, 888, recon.workers)
		require.Equal(t, 999*time.Second, recon.timeout)
	})

}
