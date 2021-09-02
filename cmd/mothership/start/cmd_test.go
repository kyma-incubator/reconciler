package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	serverPort  = 8080
	clusterName = "e2etest-cluster"
)

type TestStruct struct {
	url            string
	requestFile    string
	expectedStatus keb.ClusterStatus
	kubeconfig     string
}

func TestReconciliation(t *testing.T) {
	test.IntegrationTest(t)

	if true {
		return //disable this test temporarily
	}

	ctx := context.Background()
	defer ctx.Done()

	go func() {
		//start the mothership reconciler
		o := NewOptions(cli.NewTestOptions(t))
		o.Port = serverPort
		o.ReconcilersCfgPath = filepath.Join("test", "component-reconcilers.json")
		o.WatchInterval = 1 * time.Second
		o.Verbose = true
		require.NoError(t, Run(ctx, o))
	}()

	time.Sleep(3 * time.Second)

	tests := []*TestStruct{
		{
			url:            fmt.Sprintf("http://localhost:%d/v1/clusters", serverPort),
			requestFile:    filepath.Join("test", "request", "create_cluster.json"),
			expectedStatus: keb.ClusterStatusReady,
			kubeconfig:     test.ReadKubeconfig(t),
		},
		{
			url:            fmt.Sprintf("http://localhost:%d/v1/clusters", serverPort),
			requestFile:    filepath.Join("test", "request", "create_cluster_invalid_kubeconfig.json"),
			expectedStatus: keb.ClusterStatusError,
		},
	}

	for _, testCase := range tests {
		responseData := sendRequest(t, testCase)
		requireClusterStatus(t, responseData.StatusURL, testCase.expectedStatus, 15*time.Second)
	}
}

func sendRequest(t *testing.T, testCase *TestStruct) *keb.HTTPClusterResponse {
	payload := requestPayload(t, testCase)
	response, err := http.Post(testCase.url, "application/json", strings.NewReader(payload))
	require.NoError(t, err)

	responseBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.NoError(t, err)

	result := &keb.HTTPClusterResponse{}
	require.NoError(t, json.Unmarshal(responseBody, result))
	return result
}

func requestPayload(t *testing.T, testCase *TestStruct) string {
	data, err := ioutil.ReadFile(testCase.requestFile)
	require.NoError(t, err)
	return string(overrideKubeConfig(t, data, testCase.kubeconfig))
}

func requireClusterStatus(t *testing.T, statusURL string, expectedStatus keb.ClusterStatus, timeout time.Duration) {
	startTime := time.Now()
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		response, err := http.Get(fmt.Sprintf("http://%s", statusURL))
		require.NoError(t, err)

		body, _ := ioutil.ReadAll(response.Body)
		require.NoError(t, response.Body.Close())

		clusterResponse := &keb.HTTPClusterResponse{}
		require.NoError(t, json.Unmarshal(body, clusterResponse))

		if clusterResponse.Status == expectedStatus {
			//cluster state achieved - we are done
			return
		}

		//stop checking for expected status if cluster is in a final state
		if clusterResponse.Status == keb.ClusterStatusReady || clusterResponse.Status == keb.ClusterStatusError {
			t.Logf("Was waiting for cluster status '%s' but cluster moved into final state '%s'",
				expectedStatus, clusterResponse.Status)
			break
		}

		//check for timeout before retrying
		if time.Since(startTime) >= timeout {
			t.Logf("Timeout reached: latest status of cluster '%s' was '%s' but expected was '%s'",
				clusterName, clusterResponse.Status, expectedStatus)
			break
		}
	}

	//timeout occurred
	t.Fail()
}

func overrideKubeConfig(t *testing.T, data []byte, overrideKubeConfig string) []byte {
	if overrideKubeConfig != "" {
		newData := make(map[string]interface{})
		require.NoError(t, json.Unmarshal(data, &newData))

		newData["kubeConfig"] = overrideKubeConfig
		result, err := json.Marshal(newData)
		require.NoError(t, err)

		return result
	}
	return data
}
