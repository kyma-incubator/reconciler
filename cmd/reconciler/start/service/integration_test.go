package cmd

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
)

func (s *ReconcilerIntegrationTestSuite) verifyPodReady(_ interface{}, testCase *reconcilerIntegrationTestCase) {
	s.expectPodInState(testCase.settings.namespace, testCase.settings.deployment, progress.ReadyState) //wait until pod is ready
}
func (s *ReconcilerIntegrationTestSuite) verifyPodTerminated(_ interface{}, testCase *reconcilerIntegrationTestCase) {
	s.expectPodInState(testCase.settings.namespace, testCase.settings.deployment, progress.TerminatedState) //wait until pod is ready
}

func (s *ReconcilerIntegrationTestSuite) verifyReconcilerErrorResponse(responseModel interface{}, _ *reconcilerIntegrationTestCase) {
	s.IsType(&reconciler.HTTPErrorResponse{}, responseModel)
}

func (s *ReconcilerIntegrationTestSuite) TestRun() {
	testCases := []reconcilerIntegrationTestCase{
		{
			name: "Invalid request: mandatory fields missing",
			settings: &ReconcilerIntegrationComponentSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady: []string{"abc", "xyz"},
				Version:         "1.2.3",
				Type:            model.OperationTypeReconcile,
				Profile:         "",
				Configuration:   nil,
				Kubeconfig:      "",
				CorrelationID:   "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{
					MaxRetries: 1,
				},
				CallbackFunc: nil,
			},
			expectedHTTPCode:  http.StatusBadRequest,
			expectedResponse:  &reconciler.HTTPErrorResponse{},
			verifyResponseFct: s.verifyReconcilerErrorResponse,
		},
		{
			name: "Install component from scratch",
			settings: &ReconcilerIntegrationComponentSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady: []string{"abc", "xyz"},
				Type:            model.OperationTypeReconcile,
				Profile:         "",
				Configuration:   nil,
				Kubeconfig:      s.kubeClient.Kubeconfig(),
				CorrelationID:   "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{
					MaxRetries: 5,
				},
			},
			expectedHTTPCode:   http.StatusOK,
			expectedResponse:   &reconciler.HTTPReconciliationResponse{},
			verifyResponseFct:  s.verifyPodReady,
			verifyCallbacksFct: s.expectSuccessfulReconciliation,
		},
		{
			name: "Install external component from scratch",
			settings: &ReconcilerIntegrationComponentSettings{
				name:       "sap-btp-operator-controller-manager",
				namespace:  "inttest-comprecon-ext", //THIS ALLOWS US TO WORK EVEN WITHOUT FINALIZERS
				version:    "0.0.0",
				deployment: "sap-btp-operator-controller-manager",
				url:        "https://github.com/kyma-incubator/sap-btp-service-operator/releases/download/v0.1.18-custom/sap-btp-operator-0.1.18.tar.gz",
			},
			model: &reconciler.Task{
				ComponentsReady: []string{"abc", "xyz"},
				Type:            model.OperationTypeReconcile,
				Profile:         "",
				Configuration:   nil,
				Kubeconfig:      s.kubeClient.Kubeconfig(),
				CorrelationID:   "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{
					MaxRetries: 5,
				},
			},
			expectedHTTPCode:   http.StatusOK,
			expectedResponse:   &reconciler.HTTPReconciliationResponse{},
			verifyResponseFct:  s.verifyPodReady,
			verifyCallbacksFct: s.expectSuccessfulReconciliation,
		},
		{
			name: "Try to apply impossible change: change api version",
			settings: &ReconcilerIntegrationComponentSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon",
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
				Kubeconfig:    s.kubeClient.Kubeconfig(),
				CorrelationID: "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{
					MaxRetries: 1,
				},
			},
			expectedHTTPCode:   http.StatusOK,
			expectedResponse:   &reconciler.HTTPReconciliationResponse{},
			verifyCallbacksFct: s.expectFailingReconciliation,
		},
		{
			name: "Try to reconcile unreachable cluster",
			settings: &ReconcilerIntegrationComponentSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady: []string{"abc", "xyz"},
				Type:            model.OperationTypeReconcile,
				Profile:         "",
				Configuration:   nil,
				Kubeconfig: func() string {
					kc, err := ioutil.ReadFile(filepath.Join("test", "kubeconfig-unreachable.yaml"))
					s.NoError(err)
					return string(kc)
				}(),
				CorrelationID: "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{
					MaxRetries: 1,
				},
			},
			expectedHTTPCode:   http.StatusOK,
			expectedResponse:   &reconciler.HTTPReconciliationResponse{},
			verifyCallbacksFct: s.expectFailingReconciliation,
		},
		{
			name: "Try to deploy defective HELM chart",
			settings: &ReconcilerIntegrationComponentSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon",
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
				Kubeconfig:    s.kubeClient.Kubeconfig(),
				CorrelationID: "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{
					MaxRetries: 1,
				},
			},
			expectedHTTPCode:   http.StatusOK,
			expectedResponse:   &reconciler.HTTPReconciliationResponse{},
			verifyCallbacksFct: s.expectFailingReconciliation,
		},
		{
			name: "Simulate non-available mothership",
			settings: &ReconcilerIntegrationComponentSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady: []string{"abc", "xyz"},
				Type:            model.OperationTypeReconcile,
				Profile:         "",
				Configuration:   nil,
				Kubeconfig:      s.kubeClient.Kubeconfig(),
				CallbackURL:     "https://127.0.0.1:12345",
				CorrelationID:   "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{
					MaxRetries: 1,
				},
			},
			expectedHTTPCode:   http.StatusOK,
			expectedResponse:   &reconciler.HTTPReconciliationResponse{},
			verifyCallbacksFct: func(callbacks []*reconciler.CallbackMessage) { s.Empty(callbacks) },
		},
		{
			name: "Delete component",
			settings: &ReconcilerIntegrationComponentSettings{
				name:       "component-1",
				namespace:  "inttest-comprecon",
				version:    "0.0.0",
				deployment: "dummy-deployment",
			},
			model: &reconciler.Task{
				ComponentsReady: []string{"abc", "xyz"},
				Type:            model.OperationTypeDelete,
				Profile:         "",
				Configuration:   nil,
				Kubeconfig:      s.kubeClient.Kubeconfig(),
				CorrelationID:   "test-correlation-id",
				ComponentConfiguration: reconciler.ComponentConfiguration{
					MaxRetries: 1,
				},
			},
			expectedHTTPCode:   http.StatusOK,
			expectedResponse:   &reconciler.HTTPReconciliationResponse{},
			verifyResponseFct:  s.verifyPodTerminated,
			verifyCallbacksFct: s.expectSuccessfulReconciliation,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		s.Run(testCase.name, func() {

			if testCase.model.Namespace == "" && testCase.settings.namespace != "" {
				testCase.model.Namespace = testCase.settings.namespace
			}
			if testCase.model.Component == "" && testCase.settings.name != "" {
				testCase.model.Component = testCase.settings.name
			}
			if testCase.model.Version == "" && testCase.settings.version != "" {
				testCase.model.Version = testCase.settings.version
			}
			if testCase.model.URL == "" && testCase.settings.url != "" {
				testCase.model.URL = testCase.settings.url
			}

			if testCase.overrideWorkerConfig != nil {
				s.options.WorkerConfig = testCase.overrideWorkerConfig
			}

			s.StartupConfiguredReconciler(testCase.settings)

			var callbackC chan *reconciler.CallbackMessage

			if testCase.model.CallbackURL == "" { //set fallback callback URL
				if testCase.verifyCallbacksFct == nil { //check if validation of callback events has to happen
					testCase.model.CallbackURL = s.callbackOnNil
				} else {
					//goland:noinspection HttpUrlsUsage
					testCase.model.CallbackURL = s.callbackUrl()

					//start mock server to catch callback events
					var server *http.Server
					server, callbackC = s.newCallbackMockServer()
					s.T().Cleanup(func() {
						s.NoError(server.Shutdown(context.Background()))
						time.Sleep(1 * time.Second) //give the server some time for graceful shutdown
					})
				}
			}

			respModel := s.post(testCase)

			if testCase.verifyResponseFct != nil {
				testCase.verifyResponseFct(respModel, &testCase)
			}

			if testCase.verifyCallbacksFct != nil {
				received := s.receiveCallbacks(callbackC)
				testCase.verifyCallbacksFct(received)
			}
		})

	}
}
