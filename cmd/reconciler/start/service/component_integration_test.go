package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type NoOpReconcileAction struct {
	WaitTime time.Duration
}

func (a *NoOpReconcileAction) Run(context *service.ActionContext) (err error) {
	context.Logger.Infof("Waiting to simulate Op...")
	time.Sleep(a.WaitTime)
	return nil
}

func (s *reconcilerIntegrationTestSuite) setDefaultValuesFromTestCase(testCase *reconcilerIntegrationTestCase) {
	if testCase.settings.namespace != "" {
		testCase.model.Namespace = testCase.settings.namespace
	}
	if testCase.settings.name != "" {
		testCase.model.Component = testCase.settings.name
	}
	if testCase.settings.version != "" {
		testCase.model.Version = testCase.settings.version
	}
	if testCase.settings.url != "" {
		testCase.model.URL = testCase.settings.url
	}
	if testCase.overrideWorkerConfig != nil {
		s.options.WorkerConfig = testCase.overrideWorkerConfig
	}
}

func (s *reconcilerIntegrationTestSuite) tooManyRequestResponseParser() responseParser {
	return func(resp *http.Response) interface{} {
		return resp.StatusCode
	}
}

func (s *reconcilerIntegrationTestSuite) responseOkParser() responseParser {
	return func(response *http.Response) interface{} {
		return s.responseCheck(http.StatusOK, &reconciler.HTTPReconciliationResponse{}, response)
	}
}

func (s *reconcilerIntegrationTestSuite) responseBadRequestParser() responseParser {
	return func(response *http.Response) interface{} {
		return s.responseCheck(http.StatusBadRequest, &reconciler.HTTPErrorResponse{}, response)
	}
}

func (s *reconcilerIntegrationTestSuite) responseCheck(expectedStatus int, expectedResponse interface{}, resp *http.Response) interface{} {
	s.Equal(expectedStatus, resp.StatusCode)
	body, ioReadErr := io.ReadAll(resp.Body)
	s.NoError(ioReadErr)
	s.NoError(resp.Body.Close())
	s.testLogger.Infof("Body received from component reconciler: %s", string(body))
	s.NoError(json.Unmarshal(body, expectedResponse))
	return expectedResponse
}

func (s *reconcilerIntegrationTestSuite) verifyTooManyRequests() testCasePostExecutionVerification {
	return func(res interface{}, testCase *reconcilerIntegrationTestCase) {
		failureCount := testCase.parallelRequests - s.workerConfig.Workers
		s.True(failureCount > 0, "parallelization has to be bigger than worker amount to conclude in too many requests")
		successCount := testCase.parallelRequests - failureCount
		for _, r := range res.([]interface{}) {
			responseStatus := r.(int)
			if responseStatus >= 200 && responseStatus < 400 {
				successCount--
			} else {
				s.Equal(http.StatusTooManyRequests, responseStatus, "status must be 429 for rate limiting")
				failureCount--
			}
		}
		s.Equal(0, failureCount, "too many request verification mismatched failure requests")
		s.Equal(0, successCount, "too many request verification mismatched success requests")
	}
}
func (s *reconcilerIntegrationTestSuite) verifyPodReady() testCasePostExecutionVerification {
	return func(_ interface{}, testCase *reconcilerIntegrationTestCase) {
		s.expectPodInState(testCase.settings.namespace, testCase.settings.deployment, progress.ReadyState) // wait until pod is ready
	}
}
func (s *reconcilerIntegrationTestSuite) verifyPodTerminated() testCasePostExecutionVerification {
	return func(_ interface{}, testCase *reconcilerIntegrationTestCase) {
		s.expectPodInState(testCase.settings.namespace, testCase.settings.deployment, progress.TerminatedState) // wait until pod is ready
	}
}

func (s *reconcilerIntegrationTestSuite) expectPodInState(namespace string, deployment string, state progress.State) {
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

func (s *reconcilerIntegrationTestSuite) expectSuccessfulReconciliation() callbackVerification {
	return func(callbacks []*reconciler.CallbackMessage) {
		for idx, callback := range callbacks { // callbacks are sorted in the sequence how they were retrieved
			if idx < len(callbacks)-1 {
				s.Equal(reconciler.StatusRunning, callback.Status)
			} else {
				s.Equal(reconciler.StatusSuccess, callback.Status)
			}
		}
	}
}

func (s *reconcilerIntegrationTestSuite) expectFailingReconciliation() callbackVerification {
	return func(callbacks []*reconciler.CallbackMessage) {
		for idx, callback := range callbacks { // callbacks are sorted in the sequence how they were retrieved
			switch idx {
			case 0:
				// first callback has to indicate a running reconciliation
				s.Equal(reconciler.StatusRunning, callback.Status)
			case len(callbacks) - 1:
				// last callback has to indicate an error
				s.Equal(reconciler.StatusError, callback.Status)
			default:
				// callbacks during the reconciliation is ongoing have to indicate a failure or running
				s.Contains([]reconciler.Status{
					reconciler.StatusFailed,
					reconciler.StatusRunning,
				}, callback.Status)
			}
		}
	}
}

func (s *reconcilerIntegrationTestSuite) TestRun() {
	testCases := []reconcilerIntegrationTestCase{
		{
			name: "No-Op-Reconcile",
			settings: &componentReconcilerIntegrationSettings{
				name:            "component-1",
				namespace:       "inttest-comprecon-no-op",
				version:         "0.0.0",
				deployment:      "dummy-deployment",
				reconcileAction: &NoOpReconcileAction{WaitTime: 2 * time.Second},
			},
			model: &reconciler.Task{
				ComponentsReady:        []string{"abc", "xyz"},
				Type:                   model.OperationTypeReconcile,
				Profile:                "",
				Configuration:          nil,
				Kubeconfig:             s.kubeClient.Kubeconfig(),
				CorrelationID:          "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{MaxRetries: 5},
			},
			responseParser:       s.responseOkParser(),
			callbackVerification: s.expectSuccessfulReconciliation(),
		},
		{
			name: "Invalid request: mandatory fields missing",
			settings: &componentReconcilerIntegrationSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon-inv-mand",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady:        []string{"abc", "xyz"},
				Version:                "1.2.3",
				Type:                   model.OperationTypeReconcile,
				Profile:                "",
				Configuration:          nil,
				Kubeconfig:             "",
				CorrelationID:          "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{MaxRetries: 1},
				CallbackFunc:           nil,
			},
			responseParser: s.responseBadRequestParser(),
		},
		{
			name: "Install component from scratch",
			settings: &componentReconcilerIntegrationSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady:        []string{"abc", "xyz"},
				Type:                   model.OperationTypeReconcile,
				Profile:                "",
				Configuration:          nil,
				Kubeconfig:             s.kubeClient.Kubeconfig(),
				CorrelationID:          "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{MaxRetries: 5},
			},
			responseParser:       s.responseOkParser(),
			verification:         s.verifyPodReady(),
			callbackVerification: s.expectSuccessfulReconciliation(),
		},
		{
			name: "Install fails because of Worker Pool Overload",
			settings: &componentReconcilerIntegrationSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon-ovrld",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady:        []string{"abc", "xyz"},
				Type:                   model.OperationTypeReconcile,
				Profile:                "",
				Configuration:          nil,
				Kubeconfig:             s.kubeClient.Kubeconfig(),
				CorrelationID:          "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{MaxRetries: 5},
			},
			parallelRequests: 10,
			responseParser:   s.tooManyRequestResponseParser(),
			verification:     s.verifyTooManyRequests(),
		},
		{
			name: "Install external component from scratch",
			settings: &componentReconcilerIntegrationSettings{
				name:            "sap-btp-operator-controller-manager",
				namespace:       "inttest-comprecon-ext",
				version:         "0.0.0",
				deployment:      "sap-btp-operator-controller-manager",
				url:             "https://github.com/kyma-incubator/sap-btp-service-operator/releases/download/v0.1.18-custom/sap-btp-operator-0.1.18.tar.gz",
				deleteAfterTest: true,
			},
			model: &reconciler.Task{
				ComponentsReady:        []string{"abc", "xyz"},
				Type:                   model.OperationTypeReconcile,
				Profile:                "",
				Configuration:          nil,
				Kubeconfig:             s.kubeClient.Kubeconfig(),
				CorrelationID:          "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{MaxRetries: 5},
			},
			responseParser:       s.responseOkParser(),
			verification:         s.verifyPodReady(),
			callbackVerification: s.expectSuccessfulReconciliation(),
		},
		{
			name: "Try to apply impossible change: change api version",
			settings: &componentReconcilerIntegrationSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon-apiv",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady: []string{"abc", "xyz"},
				Type:            model.OperationTypeReconcile,
				Profile:         "",
				Configuration: map[string]interface{}{
					"v1k": "true",
				},
				Kubeconfig:             s.kubeClient.Kubeconfig(),
				CorrelationID:          "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{MaxRetries: 1},
			},
			responseParser:       s.responseOkParser(),
			callbackVerification: s.expectFailingReconciliation(),
		},
		{
			name: "Try to reconcile unreachable cluster",
			settings: &componentReconcilerIntegrationSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon-unreach",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady: []string{"abc", "xyz"},
				Type:            model.OperationTypeReconcile,
				Profile:         "",
				Configuration:   nil,
				Kubeconfig: func() string {
					kc, err := os.ReadFile(filepath.Join("test", "kubeconfig-unreachable.yaml"))
					s.NoError(err)
					return string(kc)
				}(),
				CorrelationID:          "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{MaxRetries: 1},
			},
			responseParser:       s.responseOkParser(),
			callbackVerification: s.expectFailingReconciliation(),
		},
		{
			name: "Try to deploy defective HELM chart",
			settings: &componentReconcilerIntegrationSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon-defect",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady: []string{"abc", "xyz"},
				Type:            model.OperationTypeReconcile,
				Profile:         "",
				Configuration: map[string]interface{}{
					"breakHelmChart": true,
				},
				Kubeconfig:             s.kubeClient.Kubeconfig(),
				CorrelationID:          "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{MaxRetries: 1},
			},
			responseParser:       s.responseOkParser(),
			callbackVerification: s.expectFailingReconciliation(),
		},
		{
			name: "Simulate non-available mothership",
			settings: &componentReconcilerIntegrationSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady:        []string{"abc", "xyz"},
				Type:                   model.OperationTypeReconcile,
				Profile:                "",
				Configuration:          nil,
				Kubeconfig:             s.kubeClient.Kubeconfig(),
				CallbackURL:            "https://127.0.0.1:12345",
				CorrelationID:          "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{MaxRetries: 1},
			},
			responseParser:       s.responseOkParser(),
			callbackVerification: func(callbacks []*reconciler.CallbackMessage) { s.Empty(callbacks) },
		},
		{
			name: "Delete component",
			settings: &componentReconcilerIntegrationSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon-del",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady:        []string{"abc", "xyz"},
				Type:                   model.OperationTypeDelete,
				Profile:                "",
				Configuration:          nil,
				Kubeconfig:             s.kubeClient.Kubeconfig(),
				CorrelationID:          "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{MaxRetries: 1},
			},
			responseParser:       s.responseOkParser(),
			verification:         s.verifyPodTerminated(),
			callbackVerification: s.expectSuccessfulReconciliation(),
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		s.Run(testCase.name, func() {
			s.setDefaultValuesFromTestCase(&testCase)
			s.startAndWaitForComponentReconciler(testCase.settings)

			var callbackC chan *reconciler.CallbackMessage

			if testCase.model.CallbackURL == "" { // set fallback callback URL
				if testCase.callbackVerification == nil { // check if validation of callback events has to happen
					testCase.model.CallbackURL = s.callbackOnNil
				} else {
					//goland:noinspection HttpUrlsUsage
					testCase.model.CallbackURL = s.callbackURL()

					// start mock server to catch callback events
					var server *http.Server
					server, callbackC = s.newCallbackMockServer()
					s.T().Cleanup(func() {
						s.NoError(server.Shutdown(context.Background()))
						time.Sleep(1 * time.Second) // give the server some time for graceful shutdown
					})
				}
			}

			respModel := s.post(testCase)

			if testCase.verification != nil {
				testCase.verification(respModel, &testCase)
			}

			if testCase.callbackVerification != nil {
				received := s.receiveCallbacks(callbackC)
				testCase.callbackVerification(received)
			}
		})

	}
}
