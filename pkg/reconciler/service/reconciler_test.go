package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"

	"github.com/stretchr/testify/require"
)

const workerTimeout = 30 * time.Second

type DummyAction struct {
	receivedVersion string
	receivedProfile string
	receivedConfig  []reconciler.Configuration
}

func (da *DummyAction) Run(version, profile string, config []reconciler.Configuration, helper *ActionContext) error {
	if helper.KubeClient != nil {
		return fmt.Errorf("kubeClient is not expected in this test case")
	}
	da.receivedVersion = version
	da.receivedProfile = profile
	da.receivedConfig = config
	return nil
}

func TestReconciler(t *testing.T) {

	t.Run("Verify fluent configuration interface", func(t *testing.T) {
		recon, err := NewComponentReconciler("unittest")
		require.NoError(t, err)

		require.NoError(t, recon.Debug())

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

		recon.WithServerConfig(9999, "sslCrtFile", "sslKeyFile")
		require.Equal(t, 9999, recon.serverConfig.port)
		require.Equal(t, "sslKeyFile", recon.serverConfig.sslKeyFile)
		require.Equal(t, "sslCrtFile", recon.serverConfig.sslCrtFile)

		recon.WithStatusUpdaterConfig(333*time.Second, 4455*time.Second)
		require.Equal(t, 333*time.Second, recon.statusUpdaterConfig.interval)
		require.Equal(t, 4455*time.Second, recon.statusUpdaterConfig.timeout)

		recon.WithProgressTrackerConfig(666*time.Second, 777*time.Second)
		require.Equal(t, 666*time.Second, recon.progressTrackerConfig.interval)
		require.Equal(t, 777*time.Second, recon.progressTrackerConfig.timeout)

		recon.WithWorkers(888, 999*time.Second)
		require.Equal(t, 888, recon.workers)
		require.Equal(t, 999*time.Second, recon.timeout)
	})

	t.Run("Filter missing component dependencies", func(t *testing.T) {
		recon, err := NewComponentReconciler("unittest")
		require.NoError(t, err)

		require.NoError(t, recon.Debug())

		recon.WithDependencies("a", "b")
		require.ElementsMatch(t, []string{"a", "b"}, recon.dependenciesMissing(&reconciler.Reconciliation{
			ComponentsReady: []string{"x", "y", "z"},
		}))
		require.ElementsMatch(t, []string{"b"}, recon.dependenciesMissing(&reconciler.Reconciliation{
			ComponentsReady: []string{"a", "y", "z"},
		}))
		require.ElementsMatch(t, []string{}, recon.dependenciesMissing(&reconciler.Reconciliation{
			ComponentsReady: []string{"a", "b", "z"},
		}))
	})
}

func TestReconcilerEnd2End(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	//create runtime context which is cancelled at the end of the method
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(1 * time.Second) //give some time for graceful shutdown
	}()

	//create reconciler
	recon, err := NewComponentReconciler("unittest")
	require.NoError(t, err)
	require.NoError(t, recon.Debug())

	//start reconciler
	go func() {
		err = recon.
			WithWorkspace("./test").
			WithWorkers(2, workerTimeout).
			WithServerConfig(9999, "", "").
			WithDependencies("abc", "xyz").
			WithRetry(1, 1*time.Second).
			WithStatusUpdaterConfig(1*time.Second, workerTimeout).
			WithProgressTrackerConfig(1*time.Second, workerTimeout).
			StartRemote(ctx)
		require.NoError(t, err)
	}()

	t.Run("Missing dependencies", func(t *testing.T) {
		//send request with which does not include all required dependencies: process fails before model gets processed
		body := post(t, "http://localhost:9999/v1/run", reconciler.Reconciliation{
			ComponentsReady: []string{"abc", "def"},
			Component:       "unittest-component",
			Namespace:       "unittest-service",
			Version:         "1.2.3",
			Profile:         "unittest",
			Configuration:   nil,
			Kubeconfig:      "xyz",
			CallbackURL:     "https://fake.url/",
			InstallCRD:      false,
			CorrelationID:   "test-correlation-id",
		}, http.StatusPreconditionRequired)

		//convert body to HTTP response model
		t.Logf("Body received: %s", string(body))
		resp := &reconciler.HTTPMissingDependenciesResponse{}
		require.NoError(t, json.Unmarshal(body, resp))
		require.Equal(t, []string{"abc", "xyz"}, resp.Dependencies.Required)
		require.Equal(t, []string{"xyz"}, resp.Dependencies.Missing)
	})

	t.Run("Invalid request", func(t *testing.T) {
		//send request with which does not include all mandatory model fields
		body := post(t, "http://localhost:9999/v1/run", reconciler.Reconciliation{
			ComponentsReady: []string{"abc", "xyz"},
			Component:       "unittest-component",
			Namespace:       "unittest-service",
			Version:         "1.2.3",
			Profile:         "",
			Configuration:   nil,
			Kubeconfig:      "",
			CallbackURL:     "",
			InstallCRD:      false,
			CorrelationID:   "test-correlation-id",
		}, http.StatusBadRequest)

		//convert body to HTTP response model
		t.Logf("Body received: %s", string(body))
		resp := &reconciler.HTTPErrorResponse{}
		require.NoError(t, json.Unmarshal(body, resp))
	})

	t.Run("Happy path", func(t *testing.T) {
		kubeClient, err := k8s.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewOptionalLogger(true))
		require.NoError(t, err)

		//cleanup old pods (before and after test runs)
		clCleaner := &clusterCleaner{
			reconciler: recon,
			kubeClient: kubeClient,
		}
		clCleaner.cleanup(t, "0.0.0", "component-1", "unittest-service")       //cleanup before test runs
		defer clCleaner.cleanup(t, "0.0.0", "component-1", "unittest-service") //cleanup after test is finished

		//send request with which does not include all required dependencies
		body := post(t, "http://localhost:9999/v1/run", reconciler.Reconciliation{
			ComponentsReady: []string{"abc", "xyz"},
			Component:       "component-1",
			Namespace:       "unittest-service",
			Version:         "0.0.0",
			Profile:         "",
			Configuration:   nil,
			Kubeconfig:      test.ReadKubeconfig(t),
			CallbackURL:     "https://httpbin.org/post",
			InstallCRD:      false,
			CorrelationID:   "test-correlation-id",
		}, http.StatusOK)
		t.Logf("Body received: %s", string(body))

		time.Sleep(workerTimeout) //wait until process context got closed

		//check that pod was created
		clientSet, err := kubeClient.Clientset()
		require.NoError(t, err)
		_, err = clientSet.CoreV1().Pods("unittest-service").Get(context.Background(), "dummy-pod", metav1.GetOptions{})
		require.NoError(t, err)
	})
}

func post(t *testing.T, url string, payload interface{}, expectedHTTPCode int) []byte {
	jsonPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	//nolint:gosec //in test cases is a dynamic URL acceptable
	resp, err := http.Post(url, "application/json",
		bytes.NewBuffer(jsonPayload))
	require.NoError(t, err)
	require.Equal(t, expectedHTTPCode, resp.StatusCode)

	//read body
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	return body
}
