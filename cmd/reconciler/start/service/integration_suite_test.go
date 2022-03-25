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
	"sync"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type reconcilerIntegrationTestSuite struct {
	suite.Suite

	containerSuite *db.ContainerTestSuite

	reconcilerHost string
	reconcilerPort int

	testContext   context.Context
	testDirectory string
	testLogger    *zap.SugaredLogger

	kubeClient       k8s.Client
	kubeClientConfig *k8s.Config
	chartFactory     chart.Factory

	callbackOnNil    string
	callbackMockPort int

	workerConfig *cliRecon.WorkerConfig

	options *cliRecon.Options

	serverStartMutex sync.Mutex
}

type componentReconcilerIntegrationSettings struct {
	name            string
	namespace       string
	version         string
	deployment      string
	url             string
	deleteAfterTest bool
	reconcileAction service.Action
}

type responseParser func(*http.Response) interface{}
type testCasePostExecutionVerification func(interface{}, *reconcilerIntegrationTestCase)
type callbackVerification func(callbacks []*reconciler.CallbackMessage)

type reconcilerIntegrationTestCase struct {
	name     string
	settings *componentReconcilerIntegrationSettings

	overrideWorkerConfig *cliRecon.WorkerConfig

	model                *reconciler.Task
	parallelRequests     int
	responseParser       responseParser
	callbackVerification callbackVerification
	verification         testCasePostExecutionVerification
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
	suite.Run(t, &reconcilerIntegrationTestSuite{
		containerSuite: cs,
		reconcilerHost: "localhost",
		testContext:    context.Background(),
		testDirectory:  "test",
		testLogger:     logger.NewLogger(true),

		callbackOnNil:    "https://httpbin.org/post",
		callbackMockPort: 11111,

		workerConfig: &cliRecon.WorkerConfig{
			Timeout: 35 * time.Second,
			Workers: 2,
		},

		reconcilerPort: 9999,

		kubeClientConfig: &k8s.Config{
			MaxRetries:       3,
			ProgressInterval: 5 * time.Second,
			ProgressTimeout:  16 * time.Second,
			RetryDelay:       2 * time.Second,
		},
	})
}

func (s *reconcilerIntegrationTestSuite) SetupSuite() {
	s.containerSuite.SetupSuite()
	s.serverStartMutex = sync.Mutex{}
	s.testLogger = logger.NewLogger(true)

	//use ./test folder as workspace
	wsf, err := chart.NewFactory(nil, s.testDirectory, s.testLogger)
	s.NoError(err)

	// TODO this currently blocks parallel execution as tests share one execution directory
	s.NoError(service.UseGlobalWorkspaceFactory(wsf))
	s.chartFactory = wsf

	kubeConfig := test.ReadKubeconfig(s.T())

	//create kubeClient (e.g. needed to verify reconciliation results)
	kubeClient, k8sErr := k8s.NewKubernetesClient(kubeConfig, s.testLogger, s.kubeClientConfig)
	s.NoError(k8sErr)

	s.kubeClient = kubeClient

	cliOptions := &cli.Options{Verbose: true}
	cliOptions.Registry, err = persistency.NewRegistry(s.containerSuite, true)
	s.NoError(err)
	o := cliRecon.NewOptions(cliOptions)
	o.ServerConfig.Port = s.reconcilerPort
	o.WorkerConfig = s.workerConfig
	o.HeartbeatSenderConfig.Interval = 2 * time.Second
	o.ProgressTrackerConfig.Interval = 1 * time.Second
	o.RetryConfig.RetryDelay = 2 * time.Second
	o.RetryConfig.MaxRetries = 3
	s.options = o
}

func (s *reconcilerIntegrationTestSuite) TearDownSuite() {
	s.containerSuite.TearDownSuite()
}

func (s *reconcilerIntegrationTestSuite) startAndWaitForComponentReconciler(settings *componentReconcilerIntegrationSettings) {
	recon, reconErr := service.NewComponentReconciler(settings.name)
	s.NoError(reconErr)
	recon = recon.Debug()

	if settings.reconcileAction != nil {
		recon = recon.WithReconcileAction(settings.reconcileAction)
	}

	s.T().Cleanup(func() {
		// this cleanup runs with Helm so it can happen that the cleanup unblocks while the finalizers are still not
		// dropped for a namespace. This can cause test failures for successive tests using the same namespace as the
		// API server will reject the request. You can circumvent this by using a different namespace.
		cleanUp := service.NewTestCleanup(recon, s.kubeClient)

		// this is needed to make sure the helm chart can be found under the right version
		version := settings.version
		if settings.url != "" {
			version = chart.GetExternalArchiveComponentHashedVersion(settings.url, settings.name)
		}

		cleanUp.RemoveKymaComponent(
			s.T(),
			version,
			settings.name,
			settings.namespace,
			settings.url,
		)

		// this is done as Cleanup of the externally downloaded component
		if settings.deleteAfterTest {
			s.NoError(s.chartFactory.Delete(version))
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
		// This is necessary in case the next test starts faster than Prometheus can garbage collect the Registration
		s.T().Cleanup(func() { prometheus.Unregister(recon.Collector()) })
		s.NoError(StartWebserver(componentReconcilerServerContext, s.options, workerPool, tracker))
	}()

	cliTest.WaitForTCPSocket(s.T(), s.reconcilerHost, s.reconcilerPort, 5*time.Second)
}

//goland:noinspection HttpUrlsUsage
func (s *reconcilerIntegrationTestSuite) reconcilerURL() string {
	//nolint:gosec //in test cases is a dynamic URL acceptable
	return fmt.Sprintf("http://%s:%v/v1/run", s.reconcilerHost, s.reconcilerPort)
}

//goland:noinspection HttpUrlsUsage
func (s *reconcilerIntegrationTestSuite) callbackURL() string {
	//nolint:gosec //in test cases is a dynamic URL acceptable
	return fmt.Sprintf("http://%s:%v/callback", s.reconcilerHost, s.callbackMockPort)
}

func (s *reconcilerIntegrationTestSuite) post(testCase reconcilerIntegrationTestCase) interface{} {
	if testCase.parallelRequests == 0 {
		testCase.parallelRequests++ // in case the test case does not define parallelrequests we want to request once.
	}

	jsonPayload, marshallErr := json.Marshal(testCase.model)
	s.NoError(marshallErr)
	url := s.reconcilerURL()
	s.testLogger.Infof("Sending post request to component reconciler (%s)", url)
	work := func(results chan<- interface{}) {
		resp, postErr := http.Post(s.reconcilerURL(), "application/json",
			bytes.NewBuffer(jsonPayload))
		s.NoError(postErr)
		results <- testCase.responseParser(resp)
	}

	resultChannel := make(chan interface{}, testCase.parallelRequests)
	allResults := make([]interface{}, testCase.parallelRequests)
	for range allResults {
		go work(resultChannel)
	}
	for i := range allResults {
		allResults[i] = <-resultChannel
	}
	close(resultChannel)

	if len(allResults) == 1 {
		return allResults[0]
	}
	return allResults
}

func (s *reconcilerIntegrationTestSuite) newCallbackMockServer() (*http.Server, chan *reconciler.CallbackMessage) {
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

func (s *reconcilerIntegrationTestSuite) receiveCallbacks(callbackC chan *reconciler.CallbackMessage) []*reconciler.CallbackMessage {
	var received []*reconciler.CallbackMessage
Loop:
	for {
		select {
		case callback := <-callbackC:
			received = append(received, callback)
			if callback.Status == reconciler.StatusError || callback.Status == reconciler.StatusSuccess {
				break Loop
			}
		case <-time.NewTimer(s.workerConfig.Timeout).C:
			s.testLogger.Infof("Timeout reached for retrieving callbacks")
			break Loop
		}
	}
	return received
}

func (s *reconcilerIntegrationTestSuite) newProgressTracker(clientSet clientgo.Interface) *progress.Tracker {
	prog, err := progress.NewProgressTracker(clientSet, logger.NewLogger(true), progress.Config{
		Interval: 1 * time.Second,
	})
	s.NoError(err)
	return prog
}
