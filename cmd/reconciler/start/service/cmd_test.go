package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	cliRecon "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	cliTest "github.com/kyma-incubator/reconciler/internal/cli/test"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/adapter"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"

	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"testing"
	"time"
)

const (
	workerTimeout = 30 * time.Second
	serverPort    = 9999
)

func TestReconciler(t *testing.T) {
	test.IntegrationTest(t)

	//create runtime context which is cancelled at the end of the method
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(1 * time.Second) //give some time for graceful shutdown
	}()

	//use ./test folder as workspace
	wsf, err := workspace.NewFactory("test", logger.NewOptionalLogger(true))
	require.NoError(t, err)
	require.NoError(t, service.UseGlobalWorkspaceFactory(wsf))

	//create reconciler
	recon, err := service.NewComponentReconciler("unittest") //register brand new component reconciler
	require.NoError(t, err)
	//configure reconciler
	require.NoError(t, recon.WithDependencies("abc", "xyz").Debug())

	//start reconciler
	go func() {
		o := cliRecon.NewOptions(cliTest.NewTestOptions(t))
		o.ServerConfig.Port = serverPort
		o.WorkerConfig.Timeout = workerTimeout
		o.HeartbeatSenderConfig.Interval = 1 * time.Second
		o.ProgressTrackerConfig.Interval = 1 * time.Second

		workerPool, err := StartComponentReconciler(ctx, o, "unittest")
		require.NoError(t, err)

		require.NoError(t, StartWebserver(ctx, o, workerPool))
	}()
	cliTest.WaitForTCPSocket(t, "localhost", serverPort, 5*time.Second)

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
		kubeClient, err := adapter.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewOptionalLogger(true), nil)
		require.NoError(t, err)

		//cleanup old pods (before and after test runs)
		cleanup := service.NewTestCleanup(recon, kubeClient)
		cleanup.RemoveKymaComponent(t, "0.0.0", "component-1", "unittest-service")       //cleanup before test runs
		defer cleanup.RemoveKymaComponent(t, "0.0.0", "component-1", "unittest-service") //cleanup after test is finished

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

		time.Sleep(workerTimeout) //wait until process context got closed before verifying the results

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
