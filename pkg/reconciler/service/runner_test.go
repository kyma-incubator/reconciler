package service

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"testing"
	"time"

	e "github.com/kyma-incubator/reconciler/pkg/error"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	ws "github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

const (
	kymaVersion           = "1.24.0" //if changed: please update also ./test/.gitignore file!
	fakeKymaVersion       = "0.0.0"
	clusterUsersComponent = "cluster-users"
	apiGatewayComponent   = "api-gateway"
	fakeComponent         = "component-1"
)

var wsf = &ws.Factory{
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

func (a *TestAction) logger() *zap.SugaredLogger {
	return logger.NewOptionalLogger(true)
}

func (a *TestAction) Run(version string, kubeClient *kubernetes.Clientset) error {
	if kubeClient == nil {
		return fmt.Errorf("kubeClient is expected but was nil")
	}

	a.logger().Debugf("Action '%s': received version '%s'", a.name, version)
	a.receivedVersion = version

	if a.delay > 0 {
		a.logger().Debugf("Action '%s': simulating delay of %v secs", a.name, a.delay.Seconds())
		time.Sleep(a.delay)
	}

	if a.fail {
		if !a.failAlways {
			a.fail = false //in next retry it won't fail again
		}
		a.logger().Debugf("Action '%s': simulating error", a.name)
		return fmt.Errorf("action '%s' is failing", a.name)
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

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion, false, "")
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

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, apiGatewayComponent, kymaVersion, false, "default")
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

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion, true, "")
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

		runner := newRunner(t, preAct, nil, postAct, 10*time.Second, 1*time.Minute)
		model := newModel(t, apiGatewayComponent, kymaVersion, true, "default")
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

		runner := newRunner(t, nil, instAct, nil, 10*time.Second, 1*time.Minute)
		model := newModel(t, clusterUsersComponent, kymaVersion, true, "")
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
		model := newModel(t, clusterUsersComponent, kymaVersion, false, "")
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
		model := newModel(t, clusterUsersComponent, kymaVersion, false, "")
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
		model := newModel(t, clusterUsersComponent, kymaVersion, false, "")
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
		model := newModel(t, fakeComponent, fakeKymaVersion, false, "")
		cbh := newCallbackHandler(t)

		//failing run
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := runner.Run(ctx, model, cbh)
		require.Error(t, err)
		require.IsType(t, &e.ContextClosedError{}, err)
	})

}

func newRunner(t *testing.T, preAct, instAct, postAct Action, interval, timeout time.Duration) *runner {
	recon, err := NewComponentReconciler("./test", true)
	require.NoError(t, err)

	recon.WithRetry(3, 1*time.Second).
		WithWorkers(5, timeout).
		WithStatusUpdaterConfig(interval, 3, 1*time.Second).
		WithPreInstallAction(preAct).
		WithInstallAction(instAct).
		WithPostInstallAction(postAct).
		WithProgressTrackerConfig(interval, timeout)

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

func newCallbackHandler(t *testing.T) callback.Handler {
	callbackHdlr, err := callback.NewLocalCallbackHandler(func(status reconciler.Status) error {
		return nil
	}, true)
	require.NoError(t, err)
	return callbackHdlr
}

func TestLabelInterceptor(t *testing.T) {
	type args struct {
		resource *unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		labels  map[string]string
	}{
		{
			name: "Resource without any labels",
			args: args{
				resource: &unstructured.Unstructured{},
			},
			wantErr: false,
			labels: map[string]string{
				reconciler.ManagedByLabel: reconciler.LabelReconcilerValue,
			},
		},
		{
			name: "Resource with labels",
			args: args{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"some-label":  "some-value",
								"some-label2": "some-value2",
							},
						},
					},
				},
			},
			wantErr: false,
			labels: map[string]string{
				"some-label":              "some-value",
				"some-label2":             "some-value2",
				reconciler.ManagedByLabel: reconciler.LabelReconcilerValue,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			l := &LabelInterceptor{}
			if err := l.Intercept(tt.args.resource); (err != nil) != tt.wantErr {
				t.Errorf("Intercept() error = %v, wantErr %v", err, tt.wantErr)
			}
			if fmt.Sprint(tt.labels) != fmt.Sprint(tt.args.resource.GetLabels()) {
				t.Errorf("Actual labels: %s aren't the same like expected labels: %s", tt.args.resource.GetLabels(), tt.labels)
			}
		})
	}
}
