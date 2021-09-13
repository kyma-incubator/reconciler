package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	cliRecon "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	cliTest "github.com/kyma-incubator/reconciler/internal/cli/test"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/adapter"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	clientgo "k8s.io/client-go/kubernetes"
	"net/http"
	"testing"
	"time"
)

const (
	componentReconcilerName = "unittest"
	workerTimeout           = 1 * time.Minute
	serverPort              = 9999
	componentName           = "component-1"
	componentNamespace      = "inttest-comprecon"
	componentVersion        = "0.0.0"
	componentPod            = "dummy-pod"
)

type testCase struct {
	name             string
	model            *reconciler.Reconciliation
	expectedHTTPCode int
	responseModel    interface{}
	url              string
	prepareFct       func(*testing.T, kubernetes.Client)
	verifyFct        func(*testing.T, interface{}, kubernetes.Client)
}

func TestReconciler(t *testing.T) {
	test.IntegrationTest(t)

	setGlobalWorkspaceFactory(t)
	kubeClient := newKubeClient(t)
	recon := newComponentReconciler(t) //register and configure a new component reconciler

	//cleanup old pods after and before test execution
	cleanup := service.NewTestCleanup(recon, kubeClient)
	cleanup.RemoveKymaComponent(t, componentVersion, componentName, componentNamespace)       //cleanup before tests are executed
	defer cleanup.RemoveKymaComponent(t, componentVersion, componentName, componentNamespace) //cleanup after tests are finished

	//start reconciler

	//create runtime context which is cancelled at the end of the test
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(1 * time.Second) //give some time for graceful shutdown
	}()
	startReconciler(ctx, t)

	runTestCases(t, kubeClient)
}

func setGlobalWorkspaceFactory(t *testing.T) {
	//use ./test folder as workspace
	wsf, err := workspace.NewFactory("test", logger.NewOptionalLogger(true))
	require.NoError(t, err)
	require.NoError(t, service.UseGlobalWorkspaceFactory(wsf))
}

func newKubeClient(t *testing.T) kubernetes.Client {
	//create kubeClient (e.g. needed to verify reconciliation results)
	kubeClient, err := adapter.NewKubernetesClient(test.ReadKubeconfig(t), logger.NewOptionalLogger(true), nil)
	require.NoError(t, err)
	return kubeClient
}

func newComponentReconciler(t *testing.T) *service.ComponentReconciler {
	//create reconciler
	recon, err := service.NewComponentReconciler(componentReconcilerName) //register brand new component reconciler
	require.NoError(t, err)
	//configure reconciler
	require.NoError(t, recon.WithDependencies("abc", "xyz").Debug())
	return recon
}

func startReconciler(ctx context.Context, t *testing.T) {
	go func() {
		o := cliRecon.NewOptions(cliTest.NewTestOptions(t))
		o.ServerConfig.Port = serverPort
		o.WorkerConfig.Timeout = workerTimeout
		o.HeartbeatSenderConfig.Interval = 1 * time.Second
		o.ProgressTrackerConfig.Interval = 1 * time.Second

		workerPool, err := StartComponentReconciler(ctx, o, componentReconcilerName)
		require.NoError(t, err)

		require.NoError(t, StartWebserver(ctx, o, workerPool))
	}()
	cliTest.WaitForTCPSocket(t, "localhost", serverPort, 5*time.Second)
}

func post(t *testing.T, testCase testCase) interface{} {
	jsonPayload, err := json.Marshal(testCase.model)
	require.NoError(t, err)

	//nolint:gosec //in test cases is a dynamic URL acceptable
	resp, err := http.Post(testCase.url, "application/json",
		bytes.NewBuffer(jsonPayload))
	require.NoError(t, err)
	require.Equal(t, testCase.expectedHTTPCode, resp.StatusCode)

	//read body
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	t.Logf("Body received: %s", string(body))

	require.NoError(t, json.Unmarshal(body, testCase.responseModel))
	return testCase.responseModel
}

func runTestCases(t *testing.T, kubeClient kubernetes.Client) {
	//execute test cases
	testCases := []testCase{
		{
			name: "Missing dependencies",
			model: &reconciler.Reconciliation{
				ComponentsReady: []string{"abc", "def"},
				Component:       componentName,
				Namespace:       componentNamespace,
				Version:         "1.2.3",
				Profile:         "unittest",
				Configuration:   nil,
				Kubeconfig:      "xyz",
				CallbackURL:     "https://fake.url/",
				InstallCRD:      false,
				CorrelationID:   "test-correlation-id",
			},
			expectedHTTPCode: http.StatusPreconditionRequired,
			responseModel:    &reconciler.HTTPMissingDependenciesResponse{},
			url:              "http://localhost:9999/v1/run",
			verifyFct: func(t *testing.T, responseModel interface{}, kubeClient kubernetes.Client) {
				resp := responseModel.(*reconciler.HTTPMissingDependenciesResponse)
				require.Equal(t, []string{"abc", "xyz"}, resp.Dependencies.Required)
				require.Equal(t, []string{"xyz"}, resp.Dependencies.Missing)
			},
		},
		{
			name: "Invalid request: mandatory fields missing",
			model: &reconciler.Reconciliation{
				ComponentsReady: []string{"abc", "xyz"},
				Component:       componentName,
				Namespace:       componentNamespace,
				Version:         "1.2.3",
				Profile:         "",
				Configuration:   nil,
				Kubeconfig:      "",
				CallbackURL:     "",
				InstallCRD:      false,
				CorrelationID:   "test-correlation-id",
			},
			expectedHTTPCode: http.StatusBadRequest,
			responseModel:    &reconciler.HTTPErrorResponse{},
			url:              "http://localhost:9999/v1/run",
			verifyFct: func(t *testing.T, responseModel interface{}, kubeClient kubernetes.Client) {
				require.IsType(t, &reconciler.HTTPErrorResponse{}, responseModel)
			},
		},
		{
			name: "Install component from scratch",
			model: &reconciler.Reconciliation{
				ComponentsReady: []string{"abc", "xyz"},
				Component:       componentName,
				Namespace:       componentNamespace,
				Version:         componentVersion,
				Profile:         "",
				Configuration:   nil,
				Kubeconfig:      test.ReadKubeconfig(t),
				CallbackURL:     "https://httpbin.org/post",
				InstallCRD:      false,
				CorrelationID:   "test-correlation-id",
			},
			expectedHTTPCode: http.StatusOK,
			responseModel:    &reconciler.HTTPReconciliationResponse{},
			url:              "http://localhost:9999/v1/run",
			verifyFct:        expectRunningPod,
		},
		{
			name: "Try to apply a change which is not allowed (add container to running pod)",
			model: &reconciler.Reconciliation{
				ComponentsReady: []string{"abc", "xyz"},
				Component:       componentName,
				Namespace:       componentNamespace,
				Version:         componentVersion,
				Profile:         "",
				Configuration: []reconciler.Configuration{
					{
						Key:   "initContainer",
						Value: true,
					},
				},
				Kubeconfig:    test.ReadKubeconfig(t),
				CallbackURL:   "https://httpbin.org/post",
				InstallCRD:    false,
				CorrelationID: "test-correlation-id",
			},
			expectedHTTPCode: http.StatusOK,
			responseModel:    &reconciler.HTTPReconciliationResponse{},
			url:              "http://localhost:9999/v1/run",
			verifyFct:        expectFailure,
		},
		//TODO: non-fixable component, non-reachable cluster, insufficient permissions on cluster, defective helm-chart
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, newTestFct(testCase, kubeClient))
	}
}

func newProgressTracker(t *testing.T, clientSet clientgo.Interface) *progress.Tracker {
	prog, err := progress.NewProgressTracker(clientSet, logger.NewOptionalLogger(true), progress.Config{
		Interval: 1 * time.Second,
	})
	require.NoError(t, err)
	return prog
}

//newTestFct is required to make the linter happy ;)
func newTestFct(testCase testCase, kubeClient kubernetes.Client) func(t *testing.T) {
	return func(t *testing.T) {
		if testCase.prepareFct != nil {
			testCase.prepareFct(t, kubeClient)
		}
		respModel := post(t, testCase)
		testCase.verifyFct(t, respModel, kubeClient)
	}
}

func expectFailure(t *testing.T, responseModel interface{}, kubeClient kubernetes.Client) {
	time.Sleep(30 * time.Second)
}

func expectRunningPod(t *testing.T, responseModel interface{}, kubeClient kubernetes.Client) {
	clientSet, err := kubeClient.Clientset()
	require.NoError(t, err)

	watchable, err := progress.NewWatchableResource("pod")
	require.NoError(t, err)

	t.Logf("Waiting for pod '%s' to reach READY state", componentPod)
	prog := newProgressTracker(t, clientSet)
	prog.AddResource(watchable, componentNamespace, componentPod)
	require.NoError(t, prog.Watch(context.TODO(), progress.ReadyState))
	t.Logf("Pod '%s' reached READY state", componentPod)
}
