package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/adapter"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	reconTest "github.com/kyma-incubator/reconciler/pkg/reconciler/test"
	ws "github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	kymaVersion           = "1.24.0" //if changed: please update also ./test/.gitignore file!
	fakeKymaVersion       = "0.0.0"
	clusterUsersComponent = "cluster-users"
	apiGatewayComponent   = "api-gateway"
	fakeComponent         = "component-1"
)

type TestAction struct {
	name            string
	receivedVersion string
	delay           time.Duration
	fail            bool
	failAlways      bool
}

func (a *TestAction) Run(version, profile string, config []reconciler.Configuration, context *ActionContext) error {
	log := logger.NewLogger(true)

	if context.KubeClient == nil {
		return fmt.Errorf("kubeClient is expected but was nil")
	}

	log.Debugf("Action '%s': received version '%s'", a.name, version)
	a.receivedVersion = version

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

	//cleanup
	cleanup := newCleanupFunc(t)
	cleanup(false)      //cleanup before test runs
	defer cleanup(true) //cleanup after test is finished

	t.Run("Run with pre-, post- and custom install-action", func(t *testing.T) {
		//create install actions
		preAct := &TestAction{
			name:  "pre-install",
			delay: 1 * time.Second,
		}
		instAct := &TestAction{
			name:  "install",
			delay: 1 * time.Second,
		}
		postAct := &TestAction{
			name:  "post-install",
			delay: 1 * time.Second,
		}

		runner := newRunner(t, preAct, instAct, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion, "")
		cbh := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, cbh)
		require.NoError(t, err)

		//all actions have to be executed
		require.Equal(t, kymaVersion, preAct.receivedVersion)
		require.Equal(t, kymaVersion, instAct.receivedVersion)
		require.Equal(t, kymaVersion, postAct.receivedVersion)
	})

	t.Run("Run with pre- and post-action but default install-action (without CRDs) for cluster-users component", func(t *testing.T) {
		//create install actions
		preAct := &TestAction{
			name:  "pre-install",
			delay: 1 * time.Second,
		}
		postAct := &TestAction{
			name:  "post-install",
			delay: 1 * time.Second,
		}

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 5*time.Minute) //long timeout required for slow Github clones
		model := newModel(t, clusterUsersComponent, kymaVersion, "")
		cbh := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, cbh)
		require.NoError(t, err)

		//all actions have to be executed
		require.Equal(t, kymaVersion, preAct.receivedVersion)
		require.Equal(t, kymaVersion, postAct.receivedVersion)
	})

	t.Run("Run with pre- and post-action but default install-action (without CRDs) for api-gateway component", func(t *testing.T) {
		//create install actions
		preAct := &TestAction{
			name:  "pre-install",
			delay: 1 * time.Second,
		}
		postAct := &TestAction{
			name:  "post-install",
			delay: 1 * time.Second,
		}

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 5*time.Minute) //long timeout required for slow Github clones
		model := newModel(t, apiGatewayComponent, kymaVersion, "default")
		cbh := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, cbh)
		require.NoError(t, err)

		//all actions have to be executed
		require.Equal(t, kymaVersion, preAct.receivedVersion)
		require.Equal(t, kymaVersion, postAct.receivedVersion)
	})

	t.Run("Run with pre- and post-action but default install-action (with CRDs) for cluster-users component", func(t *testing.T) {
		//create install actions
		preAct := &TestAction{
			name:  "pre-install",
			delay: 1 * time.Second,
		}
		postAct := &TestAction{
			name:  "post-install",
			delay: 1 * time.Second,
		}

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 5*time.Minute) //long timeout required for slow Github clones
		model := newModel(t, clusterUsersComponent, kymaVersion, "")
		cbh := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, cbh)
		require.NoError(t, err)

		//all actions have to be executed
		require.Equal(t, kymaVersion, preAct.receivedVersion)
		require.Equal(t, kymaVersion, postAct.receivedVersion)
	})

	t.Run("Run with pre- and post-action but default install-action (with CRDs) for api-gateway component", func(t *testing.T) {
		//create install actions
		preAct := &TestAction{
			name:  "pre-install",
			delay: 1 * time.Second,
		}
		postAct := &TestAction{
			name:  "post-install",
			delay: 1 * time.Second,
		}

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 5*time.Minute) //long timeout required for slow Github clones
		model := newModel(t, apiGatewayComponent, kymaVersion, "default")
		cbh := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, cbh)
		require.NoError(t, err)

		//all actions have to be executed
		require.Equal(t, kymaVersion, preAct.receivedVersion)
		require.Equal(t, kymaVersion, postAct.receivedVersion)
	})

	t.Run("Run without pre- and post-action", func(t *testing.T) {
		//create install actions
		instAct := &TestAction{
			name:  "install",
			delay: 1 * time.Second,
		}

		runner := newRunner(t, nil, instAct, nil, 10*time.Second, 5*time.Minute) //long timeout required for slow Github clones
		model := newModel(t, clusterUsersComponent, kymaVersion, "")
		cbh := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, cbh)
		require.NoError(t, err)

		//install action has to be executed
		require.Equal(t, kymaVersion, instAct.receivedVersion)
	})

	t.Run("Run with permanently failing pre-action", func(t *testing.T) {
		//create install actions
		preAct := &TestAction{
			name:       "pre-install",
			delay:      1 * time.Second,
			failAlways: true,
			fail:       true,
		}

		runner := newRunner(t, preAct, nil, nil, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion, "")
		cbh := newCallbackHandler(t)

		//failing run
		err := runner.Run(context.Background(), model, cbh)
		require.Error(t, err)

		//pre-install action has to be executed
		require.Equal(t, kymaVersion, preAct.receivedVersion)
	})

	t.Run("Run with permanently failing install-action", func(t *testing.T) {
		//create install actions
		install := &TestAction{
			name:       "install",
			delay:      1 * time.Second,
			failAlways: true,
			fail:       true,
		}

		runner := newRunner(t, nil, install, nil, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion, "")
		cbh := newCallbackHandler(t)

		//failing run
		err := runner.Run(context.Background(), model, cbh)
		require.Error(t, err)

		//install action has to be executed
		require.Equal(t, kymaVersion, install.receivedVersion)
	})

	t.Run("Run with permanently failing post-action", func(t *testing.T) {
		//create install actions
		install := &TestAction{
			name:  "install",
			delay: 1 * time.Second,
		}
		postAct := &TestAction{
			name:       "post-install",
			delay:      1 * time.Second,
			failAlways: true,
			fail:       true,
		}

		runner := newRunner(t, nil, install, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion, "")
		cbh := newCallbackHandler(t)

		//failing run
		err := runner.Run(context.Background(), model, cbh)
		require.Error(t, err)

		//install and post-install action have to be executed
		require.Equal(t, kymaVersion, install.receivedVersion)
		require.Equal(t, kymaVersion, postAct.receivedVersion)
	})

	t.Run("Run with exceeded timeout", func(t *testing.T) {
		runner := newRunner(t, nil, nil, nil, 1*time.Second, 2*time.Second)
		model := newModel(t, fakeComponent, fakeKymaVersion, "")
		cbh := newCallbackHandler(t)

		//failing run
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := runner.Run(ctx, model, cbh)
		require.Error(t, err)
		require.WithinDuration(t, time.Now(), start, 1500*time.Millisecond)
	})

}

func newRunner(t *testing.T, preAct, instAct, postAct Action, interval, timeout time.Duration) *runner {
	recon, err := NewComponentReconciler("unittest")
	require.NoError(t, err)

	recon.Debug().
		WithWorkspace("./test").
		WithRetry(3, 1*time.Second).
		WithWorkers(5, timeout).
		WithHeartbeatSenderConfig(interval, timeout).
		WithPreReconcileAction(preAct).
		WithReconcileAction(instAct).
		WithPostReconcileAction(postAct).
		WithProgressTrackerConfig(interval, timeout)

	return &runner{recon, logger.NewLogger(true)}
}

func newCleanupFunc(t *testing.T) func(bool) {
	recon, err := NewComponentReconciler("unittest")
	require.NoError(t, err)

	recon.Debug().WithWorkspace("./test") //use test-subfolder to cache Kyma sources

	kubeClient, err := adapter.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewLogger(true), nil)
	require.NoError(t, err)

	cleanup := NewTestCleanup(recon, kubeClient)

	return func(deleteWorkspace bool) {
		//remove all installed components
		cleanup.RemoveKymaComponent(t, kymaVersion, clusterUsersComponent, "default")
		cleanup.RemoveKymaComponent(t, kymaVersion, apiGatewayComponent, "default")
		cleanup.RemoveKymaComponent(t, fakeKymaVersion, fakeComponent, "unittest-service")
		//remove the cloned workspace
		if deleteWorkspace {
			wsf, err := ws.NewFactory(nil, "./test", logger.NewLogger(true))

			require.NoError(t, err)

			require.NoError(t, wsf.Delete(kymaVersion))
		}
	}
}

func newModel(t *testing.T, kymaComponent, kymaVersion string, namespace string) *reconciler.Reconciliation {
	return &reconciler.Reconciliation{
		Component:  kymaComponent,
		Version:    kymaVersion,
		Kubeconfig: test.ReadKubeconfig(t),
		Namespace:  namespace,
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
