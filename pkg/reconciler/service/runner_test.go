package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/adapter"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	reconTest "github.com/kyma-incubator/reconciler/pkg/reconciler/test"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	kymaVersion           = "1.24.7" //if changed: please update also ./test/.gitignore file!
	fakeKymaVersion       = "0.0.0"
	clusterUsersComponent = "cluster-users"
	fakeComponent         = "component-1"
	workspaceInHomeDir    = "reconciliation-test"
	workspaceInProjectDir = "test"
)

type TestAction struct {
	name            string
	receivedVersion string
	delay           time.Duration
	fail            bool
	failAlways      bool
}

func (a *TestAction) Run(context *ActionContext) error {
	log := logger.NewLogger(true)

	if context.KubeClient == nil {
		return fmt.Errorf("kubeClient is expected but was nil")
	}

	log.Debugf("Action '%s': received version '%s'", a.name, context.Task.Version)
	a.receivedVersion = context.Task.Version

	if a.delay > 0 {
		log.Debugf("Action '%s': simulating delay of %v secs", a.name, a.delay.Seconds())
		time.Sleep(a.delay)
	}

	if a.fail {
		if !a.failAlways {
			a.fail = false //in next retry it won't fail again
		}
		log.Debugf("Action '%s': simulating error", a.name)
		return fmt.Errorf("action '%s' is failing", a.name)
	}

	return nil
}

func TestRunner(t *testing.T) {
	test.IntegrationTest(t)

	cleanup(t)
	defer func() {
		cleanup(t)
	}()

	t.Run("Run with pre-, post- and custom reconcile-action", func(t *testing.T) {
		SetWorkspaceFactoryForHomeDir(t)

		//create actions
		preAct := &TestAction{
			name:  "pre",
			delay: 1 * time.Second,
		}
		reconcileAct := &TestAction{
			name:  "reconcile",
			delay: 1 * time.Second,
		}
		postAct := &TestAction{
			name:  "post",
			delay: 1 * time.Second,
		}

		runner := newRunner(t, preAct, reconcileAct, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion)
		cbh := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, cbh)
		require.NoError(t, err)

		//all actions have to be executed
		require.Equal(t, kymaVersion, preAct.receivedVersion)
		require.Equal(t, kymaVersion, reconcileAct.receivedVersion)
		require.Equal(t, kymaVersion, postAct.receivedVersion)
	})

	t.Run("Run with pre- and post-action but default reconcile-action for cluster-users component", func(t *testing.T) {
		SetWorkspaceFactoryForHomeDir(t)

		//create actions
		preAct := &TestAction{
			name:  "pre",
			delay: 1 * time.Second,
		}
		postAct := &TestAction{
			name:  "post",
			delay: 1 * time.Second,
		}

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 8*time.Minute) //long timeout required for slow Github clones
		model := newModel(t, clusterUsersComponent, kymaVersion)
		cbh := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, cbh)
		require.NoError(t, err)

		//all actions have to be executed
		require.Equal(t, kymaVersion, preAct.receivedVersion)
		require.Equal(t, kymaVersion, postAct.receivedVersion)
	})

	t.Run("Run with reconcile-action for cluster-users component", func(t *testing.T) {
		SetWorkspaceFactoryForHomeDir(t)

		//create reconcile action
		reconcileAct := &TestAction{
			name:  "reconcile",
			delay: 1 * time.Second,
		}

		runner := newRunner(t, nil, reconcileAct, nil, 10*time.Second, 8*time.Minute) //long timeout required for slow Github clones
		model := newModel(t, clusterUsersComponent, kymaVersion)
		cbh := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, cbh)
		require.NoError(t, err)

		//reconcile action has to be executed
		require.Equal(t, kymaVersion, reconcileAct.receivedVersion)
	})

	t.Run("Run with permanently failing pre-action for cluster-users component", func(t *testing.T) {
		SetWorkspaceFactoryForHomeDir(t)

		//create actions
		preAct := &TestAction{
			name:       "pre",
			delay:      1 * time.Second,
			failAlways: true,
			fail:       true,
		}

		runner := newRunner(t, preAct, nil, nil, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion)
		cbh := newCallbackHandler(t)

		//failing run
		err := runner.Run(context.Background(), model, cbh)
		require.Error(t, err)

		//pre action has to be executed
		require.Equal(t, kymaVersion, preAct.receivedVersion)
	})

	t.Run("Run with permanently failing reconcile-action for cluster-users component", func(t *testing.T) {
		SetWorkspaceFactoryForHomeDir(t)

		//create actions
		reconcileAct := &TestAction{
			name:       "reconcile",
			delay:      1 * time.Second,
			failAlways: true,
			fail:       true,
		}

		runner := newRunner(t, nil, reconcileAct, nil, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion)
		cbh := newCallbackHandler(t)

		//failing run
		err := runner.Run(context.Background(), model, cbh)
		require.Error(t, err)

		//reconcile action has to be executed
		require.Equal(t, kymaVersion, reconcileAct.receivedVersion)
	})

	t.Run("Run with permanently failing post-action for cluster-users component", func(t *testing.T) {
		SetWorkspaceFactoryForHomeDir(t)

		//create actions
		reconcileAct := &TestAction{
			name:  "reconcile",
			delay: 1 * time.Second,
		}
		postAct := &TestAction{
			name:       "post",
			delay:      1 * time.Second,
			failAlways: true,
			fail:       true,
		}

		runner := newRunner(t, nil, reconcileAct, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion)
		cbh := newCallbackHandler(t)

		//failing run
		err := runner.Run(context.Background(), model, cbh)
		require.Error(t, err)

		//reconcile and post action have to be executed
		require.Equal(t, kymaVersion, reconcileAct.receivedVersion)
		require.Equal(t, kymaVersion, postAct.receivedVersion)
	})

	t.Run("Run with exceeded timeout", func(t *testing.T) {
		wsf, err := workspace.NewFactory(nil, workspaceInProjectDir, logger.NewLogger(true))
		require.NoError(t, err)
		require.NoError(t, RefreshGlobalWorkspaceFactory(wsf))

		runner := newRunner(t, nil, nil, nil, 1*time.Second, 2*time.Second)
		model := newModel(t, fakeComponent, fakeKymaVersion)
		cbh := newCallbackHandler(t)

		//failing run
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err = runner.Run(ctx, model, cbh)
		require.Error(t, err)
		require.WithinDuration(t, time.Now(), start, 1500*time.Millisecond)
	})

}

func SetWorkspaceFactoryForHomeDir(t *testing.T) {
	dirname, err := os.UserHomeDir()
	require.NoError(t, err)
	wsf, err := workspace.NewFactory(nil, filepath.Join(dirname, workspaceInHomeDir), logger.NewLogger(true))
	require.NoError(t, err)
	require.NoError(t, RefreshGlobalWorkspaceFactory(wsf))
}

func newRunner(t *testing.T, preAct, reconcileAct, postAct Action, interval, timeout time.Duration) *runner {
	recon, err := NewComponentReconciler("unittest")
	require.NoError(t, err)

	dirname, err := os.UserHomeDir()
	require.NoError(t, err)
	workspaceDir := filepath.Join(dirname, workspaceInHomeDir)

	recon.Debug().
		WithWorkspace(workspaceDir).
		WithRetry(3, 1*time.Second).
		WithWorkers(5, timeout).
		WithHeartbeatSenderConfig(interval, timeout).
		WithPreReconcileAction(preAct).
		WithReconcileAction(reconcileAct).
		WithPostReconcileAction(postAct).
		WithProgressTrackerConfig(interval, timeout)

	newLogger := logger.NewLogger(true)
	return &runner{recon, NewInstall(newLogger), newLogger}
}

func cleanup(t *testing.T) {
	recon, err := NewComponentReconciler("unittest")
	require.NoError(t, err)

	kubeClient, err := adapter.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), nil)
	require.NoError(t, err)

	dirname, err := os.UserHomeDir()
	require.NoError(t, err)
	wsf, err := workspace.NewFactory(nil, filepath.Join(dirname, workspaceInHomeDir), logger.NewLogger(true))
	require.NoError(t, err)
	require.NoError(t, RefreshGlobalWorkspaceFactory(wsf))

	cleanup := NewTestCleanup(recon, kubeClient)
	cleanup.RemoveKymaComponent(t, kymaVersion, clusterUsersComponent, "default")

	wsf, err = workspace.NewFactory(nil, workspaceInProjectDir, logger.NewLogger(true))
	require.NoError(t, err)
	require.NoError(t, RefreshGlobalWorkspaceFactory(wsf))
	cleanup = NewTestCleanup(recon, kubeClient)
	cleanup.RemoveKymaComponent(t, fakeKymaVersion, fakeComponent, "default")
}

func newModel(t *testing.T, kymaComponent, kymaVersion string) *reconciler.Task {
	return &reconciler.Task{
		Component:  kymaComponent,
		Version:    kymaVersion,
		Kubeconfig: test.ReadKubeconfig(t),
		//global parameters - required by some Kyma components
		Configuration: reconTest.NewGlobalComponentConfiguration(),
	}
}

func newCallbackHandler(t *testing.T) callback.Handler {
	callbackHdlr, err := callback.NewLocalCallbackHandler(func(msg *reconciler.CallbackMessage) error {
		return nil
	}, logger.NewLogger(true))
	require.NoError(t, err)
	return callbackHdlr
}
