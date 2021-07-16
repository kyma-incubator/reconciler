package compreconciler

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/chart"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/kyma-incubator/reconciler/pkg/workspace"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"os"
	"testing"
	"time"
)

const kymaVersion = "1.2.3"

type TestAction struct {
	name            string
	receivedVersion string
	delay           time.Duration
	fail            bool
	failAlways      bool
}

func (a *TestAction) logger() *zap.Logger {
	return newLogger(true)
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

	runner := newRunner(t, preAct, instAct, postAct)
	model := newModel(t)
	callback := newCallbackHandler(t)

	//successful run
	err := runner.Run(context.Background(), model, callback)
	require.NoError(t, err)

	//all actions have to be executed
	require.Equal(t, kymaVersion, preAct.receivedVersion)
	require.Equal(t, kymaVersion, instAct.receivedVersion)
	require.Equal(t, kymaVersion, postAct.receivedVersion)
}

func newRunner(t *testing.T, preAct, instAct, postAct Action) *runner {
	chartProvider, err := chart.NewProvider(&workspace.Factory{
		StorageDir: "./test",
	}, true)
	require.NoError(t, err)

	recon := NewComponentReconciler(chartProvider).
		Debug().
		Configure(1*time.Second, 3, 1*time.Second).
		WithPreInstallAction(preAct).
		WithInstallAction(instAct).
		WithPostInstallAction(postAct)
	require.NoError(t, err)

	return &runner{recon}
}

func newModel(t *testing.T) *Reconciliation {
	//create model
	kubecfgFile := os.Getenv("KUBECONFIG")
	if !file.Exists(kubecfgFile) {
		require.FailNow(t, "Please set env-var KUBECONFIG before executing this test case")
	}
	kubecfg, err := ioutil.ReadFile(kubecfgFile)
	require.NoError(t, err)

	return &Reconciliation{
		Component:  "UnittestComponent",
		Version:    kymaVersion,
		Kubeconfig: string(kubecfg),
	}
}

func newCallbackHandler(t *testing.T) CallbackHandler {
	callbackHdlr, err := newLocalCallbackHandler(func(status Status) error {
		return nil
	}, true)
	require.NoError(t, err)
	return callbackHdlr
}
