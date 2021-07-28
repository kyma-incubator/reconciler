package service

import (
	"context"
	"fmt"
	e "github.com/kyma-incubator/reconciler/pkg/error"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/chart"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/kyma-incubator/reconciler/pkg/workspace"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

const (
	kymaVersion           = "1.24.0"
	fakeKymaVersion       = "0.0.0"
	clusterUsersComponent = "cluster-users"
	apiGatewayComponent   = "api-gateway"
	fakeComponent         = "component-1"
)

var wsf = &workspace.Factory{
	StorageDir: "./test",
	Debug:      true,
}

type TestAction struct {
	name            string
	receivedVersion string
	delay           time.Duration
	fail            bool
	failAlways      bool
}

func (a *TestAction) logger() *zap.Logger {
	return logger.NewOptionalLogger(true)
}

func (a *TestAction) Run(version string, kubeClient *kubernetes.Clientset) error {
	if kubeClient == nil {
		return fmt.Errorf("kubeClient is expected but was nil")
	}

	a.logger().Debug(fmt.Sprintf("Action '%s': received version '%s'", a.name, version))
	a.receivedVersion = version

	if a.delay > 0 {
		a.logger().Debug(fmt.Sprintf("Action '%s': simulating delay of %v secs", a.name, a.delay.Seconds()))
		time.Sleep(a.delay)
	}

	if a.fail {
		if !a.failAlways {
			a.fail = false //in next retry it won't fail again
		}
		a.logger().Debug(fmt.Sprintf("Action '%s': simulating error", a.name))
		return fmt.Errorf(fmt.Sprintf("action '%s' is failing", a.name))
	}

	return nil
}

func TestRunner(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	//cleanup relicts from previous run
	require.NoError(t, wsf.Delete(kymaVersion))
	//cleanup at the end of the run
	defer func() {
		require.NoError(t, wsf.Delete(kymaVersion))
	}()

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
		model := newModel(t, clusterUsersComponent, kymaVersion, false, "")
		callback := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, callback)
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

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion, false, "")
		callback := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, callback)
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

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, apiGatewayComponent, kymaVersion, false, "default")
		callback := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, callback)
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

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion, true, "")
		callback := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, callback)
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

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, apiGatewayComponent, kymaVersion, true, "default")
		callback := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, callback)
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

		runner := newRunner(t, nil, instAct, nil, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion, true, "")
		callback := newCallbackHandler(t)

		//successful run
		err := runner.Run(context.Background(), model, callback)
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
		model := newModel(t, clusterUsersComponent, kymaVersion, false, "")
		callback := newCallbackHandler(t)

		//failing run
		err := runner.Run(context.Background(), model, callback)
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
		model := newModel(t, clusterUsersComponent, kymaVersion, false, "")
		callback := newCallbackHandler(t)

		//failing run
		err := runner.Run(context.Background(), model, callback)
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
		model := newModel(t, clusterUsersComponent, kymaVersion, false, "")
		callback := newCallbackHandler(t)

		//failing run
		err := runner.Run(context.Background(), model, callback)
		require.Error(t, err)

		//install and post-install action have to be executed
		require.Equal(t, kymaVersion, install.receivedVersion)
		require.Equal(t, kymaVersion, postAct.receivedVersion)
	})

	t.Run("Run with exceeded timeout", func(t *testing.T) {
		runner := newRunner(t, nil, nil, nil, 1*time.Second, 2*time.Second)
		model := newModel(t, fakeComponent, fakeKymaVersion, false, "")
		callback := newCallbackHandler(t)

		//failing run
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := runner.Run(ctx, model, callback)
		require.Error(t, err)
		require.IsType(t, &e.ContextClosedError{}, err)
	})

}

func newRunner(t *testing.T, preAct, instAct, postAct Action, interval, timeout time.Duration) *runner {
	chartProvider, err := chart.NewProvider(wsf, true)
	require.NoError(t, err)

	recon := NewComponentReconciler(chartProvider).
		Debug().
		WithRetry(3, 1*time.Second).
		WithWorkers(5, timeout).
		WithStatusUpdaterConfig(interval, 3, 1*time.Second).
		WithPreInstallAction(preAct).
		WithInstallAction(instAct).
		WithPostInstallAction(postAct).
		WithProgressTrackerConfig(interval, timeout)
	require.NoError(t, err)

	return &runner{recon}
}

func newModel(t *testing.T, kymaComponent, kymaVersion string, installCRD bool, namespace string) *reconciler.Reconciliation {
	return &reconciler.Reconciliation{
		InstallCRD: installCRD,
		Component:  kymaComponent,
		Version:    kymaVersion,
		Kubeconfig: test.ReadKubeconfig(t),
		Namespace:  namespace,
	}
}

func newCallbackHandler(t *testing.T) callback.CallbackHandler {
	callbackHdlr, err := callback.NewLocalCallbackHandler(func(status reconciler.Status) error {
		return nil
	}, true)
	require.NoError(t, err)
	return callbackHdlr
}
