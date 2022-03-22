package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/kyma-incubator/reconciler/internal/cli"
	cliRecon "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	cliTest "github.com/kyma-incubator/reconciler/internal/cli/test"
	"github.com/kyma-incubator/reconciler/internal/persistency"
	"github.com/kyma-incubator/reconciler/pkg/db"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"io/ioutil"
	clientgo "k8s.io/client-go/kubernetes"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type ReconcilerIntegrationTestSuite struct {
	suite.Suite

	containerSuite *db.ContainerTestSuite

	reconcilerHost string
	reconcilerPort int

	testContext   context.Context
	testDirectory string
	testLogger    *zap.SugaredLogger

	kubeClient k8s.Client

	callbackOnNil    string
	callbackMockPort int
	urlCallbackMock  string

	workerTimeout time.Duration

	options *cliRecon.Options

	serverStartMutex sync.Mutex
}

type ReconcilerIntegrationComponentSettings struct {
	name       string
	namespace  string
	version    string
	deployment string
	url        string
}

func TestIntegrationSuite(t *testing.T) {
	containerSettings := &db.PostgresContainerSettings{
		Name:              "default-db-shared",
		Image:             "postgres:11-alpine",
		Config:            db.MigrationConfig(filepath.Join("..", "..", "..", "..", "configs", "db", "postgres")),
		Host:              "127.0.0.1",
		Database:          "kyma",
		Port:              5432,
		User:              "kyma",
		Password:          "kyma",
		EncryptionKeyFile: filepath.Join("..", "..", "..", "..", "configs", "encryption", "unittest.key"),
	}
	cs := db.LeaseSharedContainerTestSuite(
		t,
		containerSettings,
		true,
		false,
	)
	cs.SetT(t)
	suite.Run(t, &ReconcilerIntegrationTestSuite{
		containerSuite: cs,
		reconcilerHost: "localhost",
		testContext:    context.Background(),
		testDirectory:  "test",
		testLogger:     logger.NewLogger(true),

		callbackOnNil:    "https://httpbin.org/post",
		callbackMockPort: 11111,

		workerTimeout: 70 * time.Second,

		reconcilerPort: 9999,
	})
	db.ReturnLeasedSharedContainerTestSuite(t, containerSettings)
}

func (s *ReconcilerIntegrationTestSuite) SetupSuite() {
	s.containerSuite.SetupSuite()
	s.serverStartMutex = sync.Mutex{}
	s.testLogger = logger.NewLogger(true)

	//use ./test folder as workspace
	wsf, err := chart.NewFactory(nil, s.testDirectory, s.testLogger)
	s.NoError(err)

	//this currently blocks parallel execution TODO
	s.NoError(service.UseGlobalWorkspaceFactory(wsf))

	kubeConfig := test.ReadKubeconfig(s.T())

	//create kubeClient (e.g. needed to verify reconciliation results)
	kubeClient, k8sErr := k8s.NewKubernetesClient(kubeConfig, s.testLogger, nil)
	s.NoError(k8sErr)

	s.kubeClient = kubeClient

	cliOptions := &cli.Options{Verbose: true}
	cliOptions.Registry, err = persistency.NewRegistry(s.containerSuite, true)
	s.NoError(err)
	o := cliRecon.NewOptions(cliOptions)
	o.ServerConfig.Port = s.reconcilerPort
	o.WorkerConfig.Timeout = s.workerTimeout
	o.HeartbeatSenderConfig.Interval = 1 * time.Second
	o.ProgressTrackerConfig.Interval = 1 * time.Second
	o.RetryConfig.RetryDelay = 2 * time.Second
	o.RetryConfig.MaxRetries = 3
	s.options = o
}

func (s *ReconcilerIntegrationTestSuite) StartupConfiguredReconciler(settings *ReconcilerIntegrationComponentSettings) {
	recon, reconErr := service.NewComponentReconciler(settings.name)
	s.NoError(reconErr)
	recon = recon.Debug()

	s.T().Cleanup(func() {
		//TODO Install external component from scratch still causes leftover and cannot be cleaned up, we will have to change the logic to either have correct external fetching into a "resources" folder or adjust test behavior
		if settings.url == "" {
			service.NewTestCleanup(recon, s.kubeClient).RemoveKymaComponent(
				s.T(),
				settings.version,
				settings.name,
				settings.namespace,
			)
			// THIS DOES NOT BLOCK UNTIL NAMESPACE IS TERMINATED
		}
	})

	componentReconcilerServerContext, cancel := context.WithCancel(s.testContext)
	s.T().Cleanup(func() {
		cancel()
		time.Sleep(1 * time.Second) //give component reconciler some time for graceful shutdown
	})
	workerPool, tracker, startErr := StartComponentReconciler(componentReconcilerServerContext, s.options, settings.name)
	s.NoError(startErr)
	go func() {
		s.T().Cleanup(func() {
			prometheus.Unregister(recon.Collector())
		})
		s.NoError(StartWebserver(componentReconcilerServerContext, s.options, workerPool, tracker))
	}()
	cliTest.WaitForTCPSocket(s.T(), s.reconcilerHost, s.reconcilerPort, 15*time.Second)
}

type reconcilerIntegrationTestCase struct {
	name     string
	settings *ReconcilerIntegrationComponentSettings

	loadTestWorkers      bool
	overrideWorkerConfig *cliRecon.WorkerConfig

	model              *reconciler.Task
	expectedHTTPCode   int
	expectedResponse   interface{}
	verifyCallbacksFct func(callbacks []*reconciler.CallbackMessage)
	verifyResponseFct  func(interface{}, *reconcilerIntegrationTestCase)
}

//goland:noinspection HttpUrlsUsage
func (s *ReconcilerIntegrationTestSuite) reconcilerUrl() string {
	//nolint:gosec //in test cases is a dynamic URL acceptable
	return fmt.Sprintf("http://%s:%v/v1/run", s.reconcilerHost, s.reconcilerPort)
}

//goland:noinspection HttpUrlsUsage
func (s *ReconcilerIntegrationTestSuite) callbackUrl() string {
	//nolint:gosec //in test cases is a dynamic URL acceptable
	return fmt.Sprintf("http://%s:%v/callback", s.reconcilerHost, s.callbackMockPort)
}

func (s *ReconcilerIntegrationTestSuite) post(testCase reconcilerIntegrationTestCase) interface{} {
	jsonPayload, marshallErr := json.Marshal(testCase.model)
	s.NoError(marshallErr)
	url := s.reconcilerUrl()
	s.testLogger.Infof("Sending post request to component reconciler (%s)", url)
	resp, postErr := http.Post(s.reconcilerUrl(), "application/json",
		bytes.NewBuffer(jsonPayload))
	s.Equal(testCase.expectedHTTPCode, resp.StatusCode)
	if testCase.expectedHTTPCode >= 200 && testCase.expectedHTTPCode < 400 {
		s.NoError(postErr)
	}
	body, ioReadErr := ioutil.ReadAll(resp.Body)
	s.NoError(ioReadErr)
	s.NoError(resp.Body.Close())

	s.testLogger.Infof("Body received from component reconciler: %s", string(body))

	s.NoError(json.Unmarshal(body, testCase.expectedResponse))
	return testCase.expectedResponse
}

func (s *ReconcilerIntegrationTestSuite) newCallbackMockServer() (*http.Server, chan *reconciler.CallbackMessage) {
	callbackC := make(chan *reconciler.CallbackMessage, 100) //don't block

	router := mux.NewRouter()
	router.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		defer s.NoError(r.Body.Close())
		s.NoError(err, "Failed to read HTTP callback message")

		s.testLogger.Infof("Callback mock server received following callback request: %s", string(body))

		callbackData := &reconciler.CallbackMessage{}
		s.NoError(json.Unmarshal(body, &callbackData))

		callbackC <- callbackData
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.callbackMockPort),
		Handler: router,
	}
	go func() {
		s.testLogger.Infof("Starting callback mock server on port %d", s.callbackMockPort)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			s.testLogger.Infof("Failed to start callback mock server: %s", err)
		}
		s.testLogger.Infof("Shutting down callback mock server")
	}()
	cliTest.WaitForTCPSocket(s.T(), "localhost", s.callbackMockPort, 3*time.Second)

	return srv, callbackC
}

func (s *ReconcilerIntegrationTestSuite) receiveCallbacks(callbackC chan *reconciler.CallbackMessage) []*reconciler.CallbackMessage {
	var received []*reconciler.CallbackMessage
Loop:
	for {
		select {
		case callback := <-callbackC:
			received = append(received, callback)
			if callback.Status == reconciler.StatusError || callback.Status == reconciler.StatusSuccess {
				break Loop
			}
		case <-time.NewTimer(s.workerTimeout).C:
			s.testLogger.Infof("Timeout reached for retrieving callbacks")
			break Loop
		}
	}
	return received
}

func (s *ReconcilerIntegrationTestSuite) expectPodInState(namespace string, deployment string, state progress.State) {
	clientSet, err := s.kubeClient.Clientset()
	s.NoError(err)

	watchable, err := progress.NewWatchableResource("deployment")
	s.NoError(err)

	s.testLogger.Infof("Waiting for deployment '%s' to reach %s state", deployment, strings.ToUpper(string(state)))
	prog := s.newProgressTracker(clientSet)
	prog.AddResource(watchable, namespace, deployment)
	s.NoError(prog.Watch(s.testContext, state))
	s.testLogger.Infof("Deployment '%s' reached %s state", deployment, strings.ToUpper(string(state)))
}

func (s *ReconcilerIntegrationTestSuite) newProgressTracker(clientSet clientgo.Interface) *progress.Tracker {
	prog, err := progress.NewProgressTracker(clientSet, logger.NewLogger(true), progress.Config{
		Interval: 1 * time.Second,
	})
	s.NoError(err)
	return prog
}

func (s *ReconcilerIntegrationTestSuite) expectSuccessfulReconciliation(callbacks []*reconciler.CallbackMessage) {
	for idx, callback := range callbacks { //callbacks are sorted in the sequence how they were retrieved
		if idx < len(callbacks)-1 {
			s.Equal(reconciler.StatusRunning, callback.Status)
		} else {
			s.Equal(reconciler.StatusSuccess, callback.Status)
		}
	}
}

func (s *ReconcilerIntegrationTestSuite) expectFailingReconciliation(callbacks []*reconciler.CallbackMessage) {
	for idx, callback := range callbacks { //callbacks are sorted in the sequence how they were retrieved
		switch idx {
		case 0:
			//first callback has to indicate a running reconciliation
			s.Equal(reconciler.StatusRunning, callback.Status)
		case len(callbacks) - 1:
			//last callback has to indicate an error
			s.Equal(reconciler.StatusError, callback.Status)
		default:
			//callbacks during the reconciliation is ongoing have to indicate a failure or running
			s.Contains([]reconciler.Status{
				reconciler.StatusFailed,
				reconciler.StatusRunning,
			}, callback.Status)
		}
	}
}
