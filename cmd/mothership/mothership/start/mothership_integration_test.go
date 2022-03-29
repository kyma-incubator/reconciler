package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/internal/cli"
	cliRecon "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	cliTest "github.com/kyma-incubator/reconciler/internal/cli/test"
	"github.com/kyma-incubator/reconciler/internal/persistency"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/callback"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/server"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type httpMethod string

const (
	httpPost   httpMethod = http.MethodPost
	httpGet    httpMethod = http.MethodGet
	httpDelete httpMethod = http.MethodDelete
	httpPut    httpMethod = http.MethodPut
)

type mothershipIntegrationTestSuite struct {
	suite.Suite

	containerSuite *db.ContainerTestSuite

	mothershipHost         string
	mothershipPort         int
	mothershipStartTimeout time.Duration

	httpClient       *http.Client
	kubeConfig       string
	kubeClient       k8s.Client
	kubeClientConfig *k8s.Config
	chartFactory     chart.Factory

	registry      *persistency.Registry
	debugRegistry bool

	testContext   context.Context
	testLogger    *zap.SugaredLogger
	testDirectory string
}

type clusterRequest func(testCase *mothershipIntegrationTestCase) interface{}
type responseCheck func(testCase *mothershipIntegrationTestCase, response interface{}) bool
type verificationStrategy func(testCase *mothershipIntegrationTestCase)
type componentReconcilerStartStrategy func(o *cliRecon.Options, bootstrap *ComponentReconcilerBootstrap)

type mothershipIntegrationTestCase struct {
	name                              string
	verificationStrategy              verificationStrategy
	schedulerConfig                   config.SchedulerConfig
	componentReconcilerConfigs        []*ComponentReconcilerBootstrap
	appendDummyFallbackBaseReconciler bool
	componentReconcilerStartStrategy  componentReconcilerStartStrategy
}

func TestIntegrationSuite(t *testing.T) {
	cs := db.LeaseSharedContainerTestSuite(
		t,
		db.DefaultSharedContainerSettings,
		false,
		false,
	)
	cs.SetT(t)
	suite.Run(t, &mothershipIntegrationTestSuite{
		containerSuite: cs,

		mothershipStartTimeout: 10 * time.Second,
		mothershipHost:         "localhost",
		mothershipPort:         8081,

		testContext:   context.Background(),
		testLogger:    logger.NewTestLogger(t),
		testDirectory: "test",

		kubeClientConfig: &k8s.Config{
			MaxRetries:       3,
			ProgressInterval: 5 * time.Second,
			ProgressTimeout:  16 * time.Second,
			RetryDelay:       2 * time.Second,
		},

		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	})
}

func (s *mothershipIntegrationTestSuite) SetupTest() {
	var err error
	s.registry, err = persistency.NewRegistry(s.containerSuite, s.debugRegistry)
	s.NoError(err)
}

func (s *mothershipIntegrationTestSuite) TearDownTest() {
	recons, err := s.registry.ReconciliationRepository().GetReconciliations(nil)
	s.NoError(err)
	schedulingIds := make([]string, len(recons))
	for i, recon := range recons {
		schedulingIds[i] = recon.SchedulingID
	}
	s.NoError(s.registry.ReconciliationRepository().RemoveReconciliationsBySchedulingID(schedulingIds))
	s.NoError(s.registry.Close())
	s.NoError(os.Remove(s.auditLogFileName()))
}

func (s *mothershipIntegrationTestSuite) TearDownSuite() {
	s.containerSuite.TearDownSuite()
}

func (s *mothershipIntegrationTestSuite) SetupSuite() {

	s.containerSuite.SetupSuite()
	var err error

	//use ./test folder as workspace
	wsf, err := chart.NewFactory(nil, s.testDirectory, s.testLogger)
	s.NoError(err)

	// TODO this currently blocks parallel execution as tests share one execution directory
	s.NoError(service.UseGlobalWorkspaceFactory(wsf))
	s.chartFactory = wsf

	s.kubeConfig = test.ReadKubeconfig(s.T())

	//create kubeClient (e.g. needed to verify reconciliation results)
	kubeClient, k8sErr := k8s.NewKubernetesClient(s.kubeConfig, s.testLogger, s.kubeClientConfig)
	s.NoError(k8sErr)
	s.kubeClient = kubeClient
}

func (s *mothershipIntegrationTestSuite) newMothershipOptionsForTestCase(testCase *mothershipIntegrationTestCase) *Options {
	cliOpts := &cli.Options{
		Verbose: false,
	}
	var err error
	cliOpts.Registry = s.registry
	s.NoError(err)
	o := NewOptions(cliOpts)
	o.BookkeeperWatchInterval = 5 * time.Second
	o.WatchInterval = 10 * time.Second
	o.PurgeEntitiesOlderThan = 5 * time.Minute
	o.CleanerInterval = 5 * time.Minute
	o.Port = s.mothershipPort

	o.Workers = 5

	o.AuditLog = true
	o.AuditLogFile = s.auditLogFileName()
	o.AuditLogTenantID = uuid.NewString()

	o.Config = &config.Config{
		Host:      s.mothershipHost,
		Port:      s.mothershipPort,
		Scheme:    "http",
		Scheduler: testCase.schedulerConfig,
	}
	return o
}

func (s *mothershipIntegrationTestSuite) auditLogFileName() string {
	return filepath.Join(s.testDirectory, "auditlog")
}

func (s *mothershipIntegrationTestSuite) testFilePayload(file string) string {
	file = filepath.Join(s.testDirectory, "requests", file) //consider requests subfolder

	data, err := ioutil.ReadFile(file)
	s.NoError(err)
	//inject kubeconfig into testFilePayload
	newData := make(map[string]interface{})
	s.NoError(json.Unmarshal(data, &newData))

	newData["kubeConfig"] = s.kubeConfig
	result, err := json.Marshal(newData)
	s.NoError(err)

	return string(result)
}

func (s *mothershipIntegrationTestSuite) sendRequest(destURL string, httpMethod httpMethod, payload string) (*http.Response, error) {
	client := s.httpClient
	s.testLogger.Debugf("Sending %s HTTP request to: %s", httpMethod, destURL)

	var response *http.Response
	var err error
	switch httpMethod {
	case httpGet:
		response, err = client.Get(destURL)
	case httpPost:
		response, err = client.Post(destURL, "application/json", strings.NewReader(payload))
	case httpPut:
		req, err := http.NewRequest(http.MethodPut, destURL, strings.NewReader(payload))
		s.NoError(err)
		response, err = client.Do(req)
		s.NoError(err)
	case httpDelete:
		req, err := http.NewRequest(http.MethodDelete, destURL, nil)
		s.NoError(err)
		response, err = client.Do(req)
		s.NoError(err)
	}
	s.NoError(err)

	respOutput, err := httputil.DumpResponse(response, true)
	s.NoError(err)
	s.testLogger.Debugf("Received HTTP response from mothership reconciler: %s", string(respOutput))

	return response, err
}

func (s *mothershipIntegrationTestSuite) getMothershipURL() string {
	//goland:noinspection HttpUrlsUsage
	return fmt.Sprintf("http://%v:%d", s.mothershipHost, s.mothershipPort)
}

func (s *mothershipIntegrationTestSuite) requestToMothership(
	destURL string, httpMethod httpMethod, payload string, expectedHTTPCode int, expectedResponseModel interface{},
) clusterRequest {
	return func(testCase *mothershipIntegrationTestCase) interface{} {
		response, err := s.sendRequest(destURL, httpMethod, payload)
		s.NoError(err)

		if expectedHTTPCode > 0 {
			if expectedHTTPCode != response.StatusCode {
				dump, err := httputil.DumpResponse(response, true)
				s.NoError(err)
				s.testLogger.Debugf(string(dump))
			}
			s.Equal(expectedHTTPCode, response.StatusCode, "Returned HTTP response code was unexpected")
		}

		responseBody, err := ioutil.ReadAll(response.Body)
		s.NoError(response.Body.Close())
		s.NoError(err)

		if expectedResponseModel == nil {
			return nil
		}
		s.NoError(json.Unmarshal(responseBody, expectedResponseModel))

		return expectedResponseModel
	}
}

func (s *mothershipIntegrationTestSuite) verifyReconciliationScheduled() responseCheck {
	return func(testCase *mothershipIntegrationTestCase, schedulingRequestResponse interface{}) bool {
		respModel := schedulingRequestResponse.(*keb.HTTPClusterResponse)
		//depending how fast the scheduler picked up the cluster for reconciling,
		//status can be either pending or reconciling
		s.True(respModel.Status == keb.StatusReconcilePending || respModel.Status == keb.StatusReconciling,
			fmt.Sprintf("Cluster status '%s' is not allowed: expected was %s or %s",
				respModel.Status, keb.StatusReconcilePending, keb.StatusReconciling),
		)
		_, err := url.Parse(respModel.StatusURL)
		s.NoError(err)
		return true
	}
}

func (s *mothershipIntegrationTestSuite) isReconciliationFinished() responseCheck {
	return func(testCase *mothershipIntegrationTestCase, schedulingRequestResponse interface{}) bool {
		respModel := schedulingRequestResponse.(*keb.HTTPClusterResponse)

		statusRes := s.requestToMothership(
			respModel.StatusURL,
			httpGet,
			"",
			200,
			&keb.HTTPClusterResponse{},
		)(testCase).(*keb.HTTPClusterResponse)

		return statusRes.Status == keb.StatusReady
	}
}

func (s *mothershipIntegrationTestSuite) checkForSuccessfulReconciliation(creationRequestFile string, opts ...retry.Option) verificationStrategy {
	return func(testCase *mothershipIntegrationTestCase) {
		payload := s.testFilePayload(creationRequestFile)
		s.NoError(json.Unmarshal([]byte(payload), &keb.Cluster{}))
		response := s.requestToMothership(
			fmt.Sprintf("%s/v1/clusters", s.getMothershipURL()),
			httpPost,
			payload,
			200,
			&keb.HTTPClusterResponse{},
		)(testCase)
		s.verifyReconciliationScheduled()(testCase, response)
		s.NoError(retry.Do(func() error {
			if finished := s.isReconciliationFinished()(testCase, response); !finished {
				return errors.Errorf("reconciliation for %s is not finished yet", testCase.name)
			}
			return nil
		}, opts...))
	}
}

func (s *mothershipIntegrationTestSuite) emptySchedulerConfig() config.SchedulerConfig {
	return config.SchedulerConfig{
		PreComponents: [][]string{
			{"placeholder-pre"},
		},
		Reconcilers:    map[string]config.ComponentReconciler{},
		DeleteStrategy: "system",
	}
}

func (s *mothershipIntegrationTestSuite) rateLimitMockServer(o *cliRecon.Options, bootstrap *ComponentReconcilerBootstrap) {
	StartMockComponentReconciler(s.testContext, s.T(), o, &rateLimitHandler{
		logger:         s.testLogger,
		rateLimitAfter: bootstrap.WorkerConfig.Workers,
		mu:             sync.Mutex{},
	})
}

type rateLimitHandler struct {
	logger         *zap.SugaredLogger
	rateLimitAfter int
	mu             sync.Mutex
}

func (r *rateLimitHandler) Handle(_ context.Context, t *testing.T, _ *server.Params, writer http.ResponseWriter, model *reconciler.Task) {
	a := require.New(t)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rateLimitAfter > 0 {
		r.rateLimitAfter--
		a.NoError(json.NewEncoder(writer).Encode(&reconciler.HTTPReconciliationResponse{}))
		go func() {
			time.Sleep(time.Second)
			h, hErr := callback.NewRemoteCallbackHandler(model.CallbackURL, r.logger)
			a.NoError(hErr)
			a.NoError(h.Callback(&reconciler.CallbackMessage{
				Error:              "",
				Manifest:           nil,
				ProcessingDuration: int(time.Second.Milliseconds()),
				RetryID:            uuid.NewString(),
				Status:             reconciler.StatusSuccess,
			}))
		}()
	} else {
		r.rateLimitAfter++
		server.SendHTTPError(writer, http.StatusTooManyRequests, &reconciler.HTTPErrorResponse{
			Error: errors.Errorf("worker pool for %s has reached it's capacity (mocked)", model.Component).Error(),
		})
	}
}

func (s *mothershipIntegrationTestSuite) realReconciler(o *cliRecon.Options, bootstrap *ComponentReconcilerBootstrap) {
	StartComponentReconciler(s.testContext, s.T(), bootstrap, s.kubeClient, s.chartFactory, o)
}

func (s *mothershipIntegrationTestSuite) TestRun() {
	testCases := []mothershipIntegrationTestCase{
		{
			name: "Create Cluster: happy path",
			componentReconcilerConfigs: []*ComponentReconcilerBootstrap{
				{
					Name:         "component-1",
					Namespace:    "intmoth-test",
					Version:      "0.0.0",
					WorkerConfig: &cliRecon.WorkerConfig{Workers: 1, Timeout: 30 * time.Second},
				},
			},
			appendDummyFallbackBaseReconciler: true,
			verificationStrategy: s.checkForSuccessfulReconciliation("create_cluster.json",
				retry.Attempts(10),
				retry.Context(s.testContext),
				retry.Delay(5*time.Second),
				retry.OnRetry(func(n uint, err error) {
					s.testLogger.Debugf("%s, retrying (retry no. %d)",
						err.Error(), n)
				}),
			),
			componentReconcilerStartStrategy: s.realReconciler,
		},
		{
			name:                              "Create Cluster: Component Reconciler Overloaded",
			appendDummyFallbackBaseReconciler: true,
			verificationStrategy: s.checkForSuccessfulReconciliation("create_cluster.json",
				retry.Attempts(10),
				retry.Context(s.testContext),
				retry.Delay(5*time.Second),
				retry.OnRetry(func(n uint, err error) {
					s.testLogger.Debugf("%s, retrying (retry no. %d)",
						err.Error(), n)
				}),
			),
			componentReconcilerStartStrategy: s.rateLimitMockServer,
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		s.Run(testCase.name, func() {
			testCase.schedulerConfig = s.emptySchedulerConfig()

			options := s.newMothershipOptionsForTestCase(&testCase)

			if testCase.appendDummyFallbackBaseReconciler {
				testCase.componentReconcilerConfigs = append(testCase.componentReconcilerConfigs, &ComponentReconcilerBootstrap{
					Name:             config.FallbackComponentReconciler,
					Namespace:        "intmoth-test",
					Version:          "0.0.0",
					ReconcilerAction: &NoOpReconcileAction{WaitTime: 1 * time.Second},
					WorkerConfig:     &cliRecon.WorkerConfig{Workers: 1, Timeout: 30 * time.Second},
				})
			}

			for i, compReconConfig := range testCase.componentReconcilerConfigs {
				o := OptionsForComponentReconciler(s.T(), s.registry)
				o.Workspace = s.testDirectory
				o.ServerConfig.Port = s.mothershipPort + i + 1
				testCase.componentReconcilerStartStrategy(o, compReconConfig)
				cliTest.WaitForTCPSocket(s.T(), s.mothershipHost, o.ServerConfig.Port, 10*time.Second)
				testCase.schedulerConfig.Reconcilers[compReconConfig.Name] = config.ComponentReconciler{
					URL: fmt.Sprintf("%s://%s:%d/v1/run", options.Config.Scheme, options.Config.Host, o.ServerConfig.Port),
				}
			}

			ctx, cancel := context.WithCancel(s.testContext)
			s.T().Cleanup(func() {
				cancel()
				time.Sleep(1 * time.Second) //Allow graceful shutdown
			})
			go func() {
				go func(ctx context.Context, o *Options) {
					err := startScheduler(ctx, o)
					if err != nil {
						panic(err)
					}
				}(ctx, options)
				s.NoError(startWebserver(ctx, options))
			}()

			cliTest.WaitForTCPSocket(s.T(), s.mothershipHost, s.mothershipPort, s.mothershipStartTimeout)
			testCase.verificationStrategy(&testCase)
		})
	}
}
