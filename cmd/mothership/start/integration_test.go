package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	cliTest "github.com/kyma-incubator/reconciler/internal/cli/test"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	serverPort             = 8080
	clusterName            = "e2etest-cluster"
	httpPost    httpMethod = http.MethodPost
	httpGet     httpMethod = http.MethodGet
	httpDelete  httpMethod = http.MethodDelete
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
		if !(respModel.Status == keb.ClusterStatusPending || respModel.Status == keb.ClusterStatusReconciling) {
			t.Logf("Cluster status '%s' is not allowed: expected was %s or %s",
				respModel.Status, keb.ClusterStatusPending, keb.ClusterStatusReconciling)
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
			statusChanges = append(statusChanges, fmt.Sprintf("%s", statusChange))
		}
		t.Logf("Received following status changes: %s", strings.Join(statusChanges, ", "))

		//verify received status changes
		require.GreaterOrEqual(t, len(respModel.StatusChanges), 1)
		require.NotEmpty(t, respModel.StatusChanges[0].Started)
		require.NotEmpty(t, respModel.StatusChanges[0].Duration)

		//cluster status list shows latest status on top... check for the expected status depending on list length
		if len(respModel.StatusChanges) == 1 {
			require.Equal(t, keb.ClusterStatusPending, respModel.StatusChanges[0].Status)
		} else {
			if keb.ClusterStatusReconciling != respModel.StatusChanges[0].Status {
				var buffer bytes.Buffer
				for _, statusChange := range respModel.StatusChanges {
					buffer.WriteRune(',')
					buffer.WriteString(string(statusChange.Status))
				}
				t.Logf("Unexpected ordering of cluster status changes: %s", buffer.String())
			}
			require.Equal(t, keb.ClusterStatusReconciling, respModel.StatusChanges[0].Status)
		}
	}
)

type httpMethod string

type testCase struct {
	name             string
	url              string
	method           httpMethod
	payload          string
	expectedHTTPCode int
	responseModel    interface{}
	verifier         func(t *testing.T, response interface{})
}

func TestMothership(t *testing.T) {
	test.IntegrationTest(t)

	ctx := context.Background()
	defer ctx.Done()

	startMothershipReconciler(ctx, t)

	baseURL := fmt.Sprintf("http://localhost:%d/v1", serverPort)

	tests := []*testCase{
		{
			name:             "Create cluster:happy path",
			url:              fmt.Sprintf("%s/%s", baseURL, "clusters"),
			method:           httpPost,
			payload:          payload(t, "create_cluster.json", test.ReadKubeconfig(t)),
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterResponse{},
			verifier:         requireClusterResponseFct,
		},
		{
			name:             "Create cluster: non-working kubeconfig",
			url:              fmt.Sprintf("%s/%s", baseURL, "clusters"),
			method:           httpPost,
			payload:          payload(t, "create_cluster_invalid_kubeconfig.json", ""),
			expectedHTTPCode: 400,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Create cluster: invalid JSON payload",
			url:              fmt.Sprintf("%s/%s", baseURL, "clusters"),
			method:           httpPost,
			payload:          payload(t, "invalid.json", ""),
			expectedHTTPCode: 400,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Create cluster: empty body",
			url:              fmt.Sprintf("%s/%s", baseURL, "clusters"),
			method:           httpPost,
			payload:          payload(t, "empty.json", ""),
			expectedHTTPCode: 400,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Get cluster status: happy path",
			url:              fmt.Sprintf("%s/%s/configs/%d/status", fmt.Sprintf("%s/%s", baseURL, "clusters"), clusterName, 1),
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterResponse{},
			verifier:         requireClusterResponseFct,
		},
		{
			name:             "Get cluster status: using non-existing cluster",
			url:              fmt.Sprintf("%s/%s/configs/%d/status", fmt.Sprintf("%s/%s", baseURL, "clusters"), "idontexist", 1),
			method:           httpGet,
			expectedHTTPCode: 404,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Get cluster status: using non-existing version",
			url:              fmt.Sprintf("%s/%s/configs/%d/status", fmt.Sprintf("%s/%s", baseURL, "clusters"), clusterName, 9999),
			method:           httpGet,
			expectedHTTPCode: 404,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Get cluster: happy path",
			url:              fmt.Sprintf("%s/%s/status", fmt.Sprintf("%s/%s", baseURL, "clusters"), clusterName),
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterResponse{},
			verifier:         requireClusterResponseFct,
		},
		{
			name:             "Get cluster: using non-existing cluster",
			url:              fmt.Sprintf("%s/%s/status", fmt.Sprintf("%s/%s", baseURL, "clusters"), "idontexist"),
			method:           httpGet,
			expectedHTTPCode: 404,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Get list of status changes: without offset",
			url:              fmt.Sprintf("%s/%s/statusChanges", fmt.Sprintf("%s/%s", baseURL, "clusters"), clusterName),
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterStatusResponse{},
			verifier:         requireClusterStatusResponseFct,
		},
		{
			name:             "Get list of status changes: with url-param offset",
			url:              fmt.Sprintf("%s/%s/statusChanges?offset=6h", fmt.Sprintf("%s/%s", baseURL, "clusters"), clusterName),
			method:           httpGet,
			expectedHTTPCode: 200,
			responseModel:    &keb.HTTPClusterStatusResponse{},
			verifier:         requireClusterStatusResponseFct,
		},
		{
			name:             "Get list of status changes: using non-existing cluster",
			url:              fmt.Sprintf("%s/%s/statusChanges?offset=6h", fmt.Sprintf("%s/%s", baseURL, "clusters"), "I dont exist"),
			method:           httpGet,
			expectedHTTPCode: 404,
			responseModel:    &keb.HTTPErrorResponse{},
			verifier:         requireErrorResponseFct,
		},
		{
			name:             "Get list of status changes: using invalid offset",
			url:              fmt.Sprintf("%s/%s/statusChanges?offset=4y", fmt.Sprintf("%s/%s", baseURL, "clusters"), clusterName),
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
	}

	for _, testCase := range tests {
		t.Run(testCase.name, newTestFct(testCase))
	}
}

//newTestFct is required to make the linter happy ;)
func newTestFct(testCase *testCase) func(t *testing.T) {
	return func(t *testing.T) {
		resp := callMothership(t, testCase)
		if testCase.verifier != nil {
			testCase.verifier(t, resp)
		}
	}
}

func startMothershipReconciler(ctx context.Context, t *testing.T) {
	go func(ctx context.Context) {
		o := NewOptions(cliTest.NewTestOptions(t))
		o.Port = serverPort
		o.ReconcilersCfgPath = filepath.Join("test", "component-reconcilers.json")
		o.WatchInterval = 1 * time.Second
		o.Verbose = true

		t.Log("Starting mothership reconciler")
		require.NoError(t, Run(ctx, o))
	}(ctx)

	cliTest.WaitForTCPSocket(t, "127.0.0.1", serverPort, 8*time.Second)
}

func callMothership(t *testing.T, testCase *testCase) interface{} {
	response, err := sendRequest(t, testCase)
	require.NoError(t, err)

	if testCase.expectedHTTPCode != response.StatusCode {
		dump, err := httputil.DumpResponse(response, true)
		require.NoError(t, err)
		t.Log(string(dump))
	}
	require.Equal(t, testCase.expectedHTTPCode, response.StatusCode, "Returned HTTP response code was unexpected")

	responseBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal(responseBody, testCase.responseModel))
	return testCase.responseModel
}

func sendRequest(t *testing.T, testCase *testCase) (*http.Response, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	var response *http.Response
	var err error
	switch testCase.method {
	case httpGet:
		response, err = client.Get(testCase.url)
	case httpPost:
		response, err = client.Post(testCase.url, "application/json", strings.NewReader(testCase.payload))
	case httpDelete:
		req, err := http.NewRequest(http.MethodDelete, testCase.url, nil)
		require.NoError(t, err)
		response, err = client.Do(req)
		require.NoError(t, err)
	}
	require.NoError(t, err)
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
