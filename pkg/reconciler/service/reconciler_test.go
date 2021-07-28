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

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/test"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
)

const workerTimeout = 10 * time.Second

type DummyAction struct {
	receivedVersion string
}

func (da *DummyAction) Run(version string, kubeClient *kubernetes.Clientset) error {
	if kubeClient != nil {
		return fmt.Errorf("kubeClient is not expected in this test case")
	}
	da.receivedVersion = version
	return nil
}

func TestReconciler(t *testing.T) {

	t.Run("Verify fluent configuration interface", func(t *testing.T) {
		recon, err := NewComponentReconciler("./test", true)
		require.NoError(t, err)
		require.True(t, recon.debug) //debug has to be enabled

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
		}
		instAct := &DummyAction{
			"123",
		}
		postAct := &DummyAction{
			"123",
		}
		recon.WithPreInstallAction(preAct).
			WithInstallAction(instAct).
			WithPostInstallAction(postAct)
		require.Equal(t, preAct, recon.preInstallAction)
		require.Equal(t, instAct, recon.installAction)
		require.Equal(t, postAct, recon.postInstallAction)

		recon.WithServerConfig(9999, "sslCrtFile", "sslKeyFile")
		require.Equal(t, 9999, recon.serverConfig.port)
		require.Equal(t, "sslKeyFile", recon.serverConfig.sslKeyFile)
		require.Equal(t, "sslCrtFile", recon.serverConfig.sslCrtFile)

		recon.WithStatusUpdaterConfig(333*time.Second, 444, 555*time.Second)
		require.Equal(t, 333*time.Second, recon.statusUpdaterConfig.interval)
		require.Equal(t, 444, recon.statusUpdaterConfig.maxRetries)
		require.Equal(t, 555*time.Second, recon.statusUpdaterConfig.retryDelay)

		recon.WithProgressTrackerConfig(666*time.Second, 777*time.Second)
		require.Equal(t, 666*time.Second, recon.progressTrackerConfig.interval)
		require.Equal(t, 777*time.Second, recon.progressTrackerConfig.timeout)

		recon.WithWorkers(888, 999*time.Second)
		require.Equal(t, 888, recon.workers)
		require.Equal(t, 999*time.Second, recon.timeout)
	})

	t.Run("Filter missing component dependencies", func(t *testing.T) {
		recon, err := NewComponentReconciler("./test", true)
		require.NoError(t, err)

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

	//start reconciler
	go func() {
		recon, err := NewComponentReconciler("./test", true)
		require.NoError(t, err)

		err = recon.WithWorkers(2, workerTimeout).
			WithServerConfig(9999, "", "").
			WithDependencies("abc", "xyz").
			WithRetry(1, 1*time.Second).
			WithStatusUpdaterConfig(1*time.Second, 1, 1*time.Second).
			WithProgressTrackerConfig(1*time.Second, workerTimeout).
			StartRemote(ctx)
		require.NoError(t, err)
	}()

	t.Run("Missing dependencies", func(t *testing.T) {
		//send request with which does not include all required dependencies: process fails before model gets processed
		body := post(t, "http://localhost:9999/v1/run", reconciler.Reconciliation{
			ComponentsReady: []string{"abc", "def"},
			Component:       "unittest-component",
			Namespace:       "unittest",
			Version:         "1.2.3",
			Profile:         "unittest",
			Configuration:   nil,
			Kubeconfig:      "xyz",
			CallbackURL:     "https://fake.url/",
			InstallCRD:      false,
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
			Namespace:       "unittest",
			Version:         "1.2.3",
			Profile:         "unittest",
			Configuration:   nil,
			Kubeconfig:      "",
			CallbackURL:     "",
			InstallCRD:      false,
		}, http.StatusBadRequest)

		//convert body to HTTP response model
		t.Logf("Body received: %s", string(body))
		resp := &reconciler.HTTPErrorResponse{}
		require.NoError(t, json.Unmarshal(body, resp))
	})

	t.Run("Happy path", func(t *testing.T) {
		//send request with which does not include all required dependencies
		body := post(t, "http://localhost:9999/v1/run", reconciler.Reconciliation{
			ComponentsReady: []string{"abc", "xyz"},
			Component:       "component-1",
			Namespace:       "default",
			Version:         "0.0.0",
			Profile:         "",
			Configuration:   nil,
			Kubeconfig:      test.ReadKubeconfig(t),
			CallbackURL:     "https://httpbin.org/post",
			InstallCRD:      false,
		}, http.StatusOK)
		t.Logf("Body received: %s", string(body))
		time.Sleep(workerTimeout) //just let it run until worker timeout occurs
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
