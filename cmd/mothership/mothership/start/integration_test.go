package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cliTest "github.com/kyma-incubator/reconciler/internal/cli/test"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

const (
	clusterName             = "e2etest-cluster"
	clusterName2            = "e2etest-cluster2"
	httpPost     httpMethod = http.MethodPost
	httpGet      httpMethod = http.MethodGet
	httpDelete   httpMethod = http.MethodDelete
)

var (
	requireErrorResponseFct = func(t *testing.T, response interface{}) {
		errModel := response.(*keb.HTTPErrorResponse)
		require.NotEmpty(t, errModel.Error)
		t.Logf("Retrieve error message: %s", errModel.Error)
	}

	requireClusterResponseFct = func(t *testing.T, response interface{}) {
		respModel := response.(*keb.HTTPClusterResponse)
		//depending how fast the scheduler picked up the cluster for reconciling,
		//status can be either pending or reconciling
		if !(respModel.Status == keb.StatusReconcilePending || respModel.Status == keb.StatusReconciling) {
			t.Logf("Cluster status '%s' is not allowed: expected was %s or %s",
				respModel.Status, keb.StatusReconcilePending, keb.StatusReconciling)
			t.Fail()
		}
		_, err := url.Parse(respModel.StatusURL)
		require.NoError(t, err)
	}

	requireClusterStatusResponseFct = func(t *testing.T, response interface{}) {
		respModel := response.(*keb.HTTPClusterStatusResponse)

		//dump received status chagnes for debugging purposes
		var statusChanges []string
		for _, statusChange := range respModel.StatusChanges {
			statusChanges = append(statusChanges, fmt.Sprintf("%+v", statusChange))
		}
		t.Logf("Received following status changes: %s", strings.Join(statusChanges, ", "))

		//verify received status changes
		require.GreaterOrEqual(t, len(respModel.StatusChanges), 1)
		require.NotEmpty(t, respModel.StatusChanges[0].Started)
		require.NotEmpty(t, respModel.StatusChanges[0].Duration)

		//cluster status list shows latest status on top... check for the expected status depending on list length
		if len(respModel.StatusChanges) == 1 {
			require.Equal(t, keb.StatusReconcilePending, respModel.StatusChanges[0].Status)
		} else {
			if keb.StatusReconcilePending != respModel.StatusChanges[0].Status {
				var buffer bytes.Buffer
				for _, statusChange := range respModel.StatusChanges {
					if buffer.Len() > 0 {
						buffer.WriteRune(',')
					}
					buffer.WriteString(string(statusChange.Status))
				}
				t.Logf("Unexpected ordering of cluster status changes: %s", buffer.String())
			}
			//check last element in slice (ordering in slice is DESC => latest event at the beginning)
			require.Equal(t, keb.StatusReconcilePending, respModel.StatusChanges[len(respModel.StatusChanges)-1].Status)
		}
	}
)

type httpMethod string

type testCase struct {
	beforeTestCase   func()
	name             string
	url              string
	dynamicURL       func() string
	method           httpMethod
	payload          string
	expectedHTTPCode int
	responseModel    interface{}
	verifier         func(t *testing.T, response interface{})
}

func TestMothership(t *testing.T) {
	test.IntegrationTest(t)

	//start mothership service
	ctx := context.Background()
	defer ctx.Done()

	serverPort := startMothershipReconciler(ctx, t)
	baseURL := fmt.Sprintf("http://localhost:%d/v1", serverPort)

	tests := []*testCase{
		{
			name:   "Delete old relicts before test starts",
			url:    fmt.Sprintf("%s/clusters/%s", baseURL, clusterName),
			method: httpDelete,
		},
		{
			name:             "Create cluster:happy path",
			url:              fmt.Sprintf("%s/clusters", baseURL),
			method:           httpPost,
			payload:          payload(t, "create_cluster.json", test.ReadKubeconfig(t)),
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterResponse{},
			verifier:         requireClusterResponseFct,
		},
		{
			name:             "Create cluster2:happy path",
			url:              fmt.Sprintf("%s/clusters", baseURL),
			method:           httpPost,
			payload:          payload(t, "create_cluster2.json", test.ReadKubeconfig(t)),
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterResponse{},
			verifier:         requireClusterResponseFct,
		},
		{
			name:             "Create cluster: non-working kubeconfig",
			url:              fmt.Sprintf("%s/clusters", baseURL),
			method:           httpPost,
			payload:          payload(t, "create_cluster_invalid_kubeconfig.json", ""),
			expectedHTTPCode: 400,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Create cluster: invalid JSON payload",
			url:              fmt.Sprintf("%s/clusters", baseURL),
			method:           httpPost,
			payload:          payload(t, "invalid.json", ""),
			expectedHTTPCode: 400,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Create cluster: empty body",
			url:              fmt.Sprintf("%s/clusters", baseURL),
			method:           httpPost,
			payload:          payload(t, "empty.json", ""),
			expectedHTTPCode: 400,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name: "Get cluster status: happy path",
			dynamicURL: func() string {
				resp := callMothership(t, &testCase{
					url:              fmt.Sprintf("%s/clusters/%s/status", baseURL, clusterName),
					method:           httpGet,
					expectedHTTPCode: 200,
					responseModel:    &keb.HTTPClusterResponse{},
				})
				respModel := resp.(*keb.HTTPClusterResponse)
				return fmt.Sprintf("%s/clusters/%s/configs/%d/status", baseURL, clusterName, respModel.ConfigurationVersion)
			},
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterResponse{},
			verifier:         requireClusterResponseFct,
		},
		{
			name:             "Get cluster status: using non-existing cluster",
			url:              fmt.Sprintf("%s/clusters/%s/configs/%d/status", baseURL, "idontexist", 1),
			method:           httpGet,
			expectedHTTPCode: 404,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Get cluster status: using non-existing version",
			url:              fmt.Sprintf("%s/clusters/%s/configs/%d/status", baseURL, clusterName, 9999),
			method:           httpGet,
			expectedHTTPCode: 404,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Get cluster: happy path",
			url:              fmt.Sprintf("%s/clusters/%s/status", baseURL, clusterName),
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterResponse{},
			verifier:         requireClusterResponseFct,
		},
		{
			name:             "Get cluster: using non-existing cluster",
			url:              fmt.Sprintf("%s/clusters/%s/status", baseURL, "idontexist"),
			method:           httpGet,
			expectedHTTPCode: 404,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Get list of status changes: without offset",
			url:              fmt.Sprintf("%s/clusters/%s/statusChanges", baseURL, clusterName),
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterStatusResponse{},
			verifier:         requireClusterStatusResponseFct,
		},
		{
			name:             "Get list of status changes: with url-param offset",
			url:              fmt.Sprintf("%s/clusters/%s/statusChanges?offset=6h", baseURL, clusterName),
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterStatusResponse{},
			verifier:         requireClusterStatusResponseFct,
		},
		{
			name:             "Get list of status changes: using non-existing cluster",
			url:              fmt.Sprintf("%s/clusters/%s/statusChanges?offset=6h", baseURL, "I-dont-exist"),
			method:           httpGet,
			expectedHTTPCode: 404,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Get list of status changes: using invalid offset",
			url:              fmt.Sprintf("%s/clusters/%s/statusChanges?offset=4y", baseURL, clusterName),
			method:           httpGet,
			expectedHTTPCode: 400,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Component reconciler heartbeat: using invalid IDs",
			url:              fmt.Sprintf("%s/%s/callback/%s", fmt.Sprintf("%s/%s", baseURL, "operations"), "opsId", "corrId"),
			payload:          payload(t, "callback.json", ""),
			method:           httpPost,
			expectedHTTPCode: 404,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Component reconciler heartbeat: using non-expected JSON payload (JSON is valid)",
			url:              fmt.Sprintf("%s/%s/callback/%s", fmt.Sprintf("%s/%s", baseURL, "operations"), "opsId", "corrId"),
			payload:          payload(t, "create_cluster.json", ""),
			method:           httpPost,
			expectedHTTPCode: 400,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Component reconciler heartbeat: without payload",
			url:              fmt.Sprintf("%s/%s/callback/%s", fmt.Sprintf("%s/%s", baseURL, "operations"), "opsId", "corrId"),
			method:           httpPost,
			expectedHTTPCode: 400,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Get list of reconciliations: all",
			url:              fmt.Sprintf("%s/reconciliations", baseURL),
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.ReconcilationsOKResponse{},
			beforeTestCase:   wait(3 * time.Second),
			verifier:         twoReconciliations,
		},
		{
			name:             "Get list of reconciliations: no runtime id",
			url:              fmt.Sprintf("%s/reconciliations?runtimeIDs=none", baseURL),
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.ReconcilationsOKResponse{},
			verifier:         zeroReconciliations,
			// no need for waiting in initFn
		},
		{
			name:             "Get list of reconciliations: filter by status",
			url:              fmt.Sprintf("%s/reconciliations?statuses=none", baseURL),
			method:           httpGet,
			expectedHTTPCode: 400,
			responseModel:    &keb.HTTPErrorResponse{},
			// no need for waiting in initFn
		},
		{
			name:             "Get list of reconciliations: filter by runtimeId",
			url:              fmt.Sprintf("%s/reconciliations?runtimeIDs=e2etest-cluster2", baseURL),
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.ReconcilationsOKResponse{},
			verifier:         oneReconciliation,
			// no need for waiting in initFn
		},
		{
			name:             "Get operation: not found",
			url:              fmt.Sprintf("%s/reconciliations/xxx/info", baseURL),
			method:           httpGet,
			expectedHTTPCode: 404,
			responseModel:    &keb.HTTPErrorResponse{},
			// no need for waiting in initFn
		},
		{
			name: "Get operation: found",
			dynamicURL: func() string {
				resp := callMothership(t, &testCase{
					url:              fmt.Sprintf("%s/reconciliations", baseURL),
					method:           httpGet,
					expectedHTTPCode: 200,
					responseModel:    &keb.ReconcilationsOKResponse{},
				})

				respModel := *(resp.(*keb.ReconcilationsOKResponse))
				if len(respModel) < 1 {
					t.Errorf("no reconciliations in db")
				}

				return fmt.Sprintf("%s/reconciliations/%s/info", baseURL, respModel[0].SchedulingID)
			},
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPReconciliationOperations{},
			verifier:         twoReconciliationOps,
			// no need for waiting in initFn
		},
		{
			name:   "Cleanup test context",
			url:    fmt.Sprintf("%s/clusters/%s", baseURL, clusterName),
			method: httpDelete,
		},
		{
			name:   "Cleanup test context2",
			url:    fmt.Sprintf("%s/clusters/%s", baseURL, clusterName2),
			method: httpDelete,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, newTestFct(testCase))
	}
}

//newTestFct is required to make the linter happy ;)
func newTestFct(testCase *testCase) func(t *testing.T) {
	return func(t *testing.T) {
		if testCase.beforeTestCase != nil {
			testCase.beforeTestCase()
		}
		resp := callMothership(t, testCase)
		if testCase.verifier != nil {
			testCase.verifier(t, resp)
		}
	}
}

func startMothershipReconciler(ctx context.Context, t *testing.T) int {
	cliTest.InitViper(t)
	serverPort := viper.GetInt("mothership.port")

	go func(ctx context.Context) {
		o := NewOptions(cliTest.NewTestOptions(t))
		o.WatchInterval = 1 * time.Second
		o.Port = serverPort
		o.Verbose = true

		t.Log("Starting mothership reconciler")
		require.NoError(t, Run(ctx, o))
	}(ctx)

	cliTest.WaitForTCPSocket(t, "127.0.0.1", serverPort, 8*time.Second)

	return serverPort
}

func callMothership(t *testing.T, testCase *testCase) interface{} {
	response, err := sendRequest(t, testCase)
	require.NoError(t, err)

	if testCase.expectedHTTPCode > 0 {
		if testCase.expectedHTTPCode != response.StatusCode {
			dump, err := httputil.DumpResponse(response, true)
			require.NoError(t, err)
			t.Log(string(dump))
		}
		require.Equal(t, testCase.expectedHTTPCode, response.StatusCode, "Returned HTTP response code was unexpected")
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.NoError(t, err)

	if testCase.responseModel == nil {
		return nil
	}
	require.NoError(t, json.Unmarshal(responseBody, testCase.responseModel))
	return testCase.responseModel
}

func sendRequest(t *testing.T, testCase *testCase) (*http.Response, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	destURL := testCase.url
	if testCase.dynamicURL != nil {
		destURL = testCase.dynamicURL()
	}

	var response *http.Response
	var err error
	switch testCase.method {
	case httpGet:
		response, err = client.Get(destURL)
	case httpPost:
		response, err = client.Post(destURL, "application/json", strings.NewReader(testCase.payload))
	case httpDelete:
		req, err := http.NewRequest(http.MethodDelete, destURL, nil)
		require.NoError(t, err)
		response, err = client.Do(req)
		require.NoError(t, err)
	}
	require.NoError(t, err)

	respOutput, err := httputil.DumpResponse(response, true)
	require.NoError(t, err)
	t.Logf("Received HTTP response from mothership reconciler: %s", string(respOutput))

	return response, err
}

func payload(t *testing.T, file, kubeconfig string) string {
	file = filepath.Join("test", "requests", file) //consider test/requests subfolder

	data, err := ioutil.ReadFile(file)
	require.NoError(t, err)

	if kubeconfig == "" {
		return string(data)
	}

	//inject kubeconfig into payload
	newData := make(map[string]interface{})
	require.NoError(t, json.Unmarshal(data, &newData))

	newData["kubeConfig"] = kubeconfig
	result, err := json.Marshal(newData)
	require.NoError(t, err)

	return string(result)
}

func wait(d time.Duration) func() {
	return func() {
		time.Sleep(d)
	}
}

type verifier = func(*testing.T, interface{})

func hasReconciliation(p func(int) bool) verifier {
	return func(t *testing.T, response interface{}) {
		var status keb.ReconcilationsOKResponse = *response.(*keb.ReconcilationsOKResponse)
		actualReconciliationSize := len(status)

		if !p(actualReconciliationSize) {
			t.Errorf("unexpected reconciliation size: %d", actualReconciliationSize)
		}
	}
}

func hasReconciliationOpt(p func(int) bool) verifier {
	return func(t *testing.T, response interface{}) {
		var result keb.HTTPReconciliationOperations = *response.(*keb.HTTPReconciliationOperations)
		actualReconciliationSize := len(*result.Operations)

		if !p(actualReconciliationSize) {
			t.Errorf("unexpected reconciliation operation size: %d", actualReconciliationSize)
		}
	}
}

var (
	zeroReconciliations  verifier = hasReconciliation(func(i int) bool { return i == 0 })
	oneReconciliation    verifier = hasReconciliation(func(i int) bool { return i == 1 })
	twoReconciliations   verifier = hasReconciliation(func(i int) bool { return i == 2 })
	twoReconciliationOps verifier = hasReconciliationOpt(func(i int) bool { return i == 2 })
)
