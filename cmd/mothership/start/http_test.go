package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/model"
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
	expectedStatus model.Status
	kubeconfig     string
}

func TestReconciliation(t *testing.T) {
	test.IntegrationTest(t)

	ctx := context.Background()
	defer ctx.Done()

	go func() {
		o := NewOptions(cli.NewTestOptions(t))
		o.Port = serverPort
		require.NoError(t, startWebserver(ctx, o))
	}()

	time.Sleep(3 * time.Second)

	tests := []*TestStruct{
		{
			url:            fmt.Sprintf("http://localhost:%d/v1/clusters", serverPort),
			requestFile:    filepath.Join("test", "request", "create_cluster.json"),
			expectedStatus: model.ClusterStatusReady,
			kubeconfig:     test.ReadKubeconfig(t),
		},
		{
			url:            fmt.Sprintf("http://localhost:%d/v1/clusters", serverPort),
			requestFile:    filepath.Join("test", "request", "create_cluster_invalid_kubeconfig.json"),
			expectedStatus: model.ClusterStatusError,
		},
	}

	for _, testCase := range tests {
		responseData := sendRequest(t, testCase)
		requireClusterStatus(t, responseData["statusUrl"].(string), testCase.expectedStatus, 15*time.Second)
	}
}

func sendRequest(t *testing.T, testCase *TestStruct) map[string]interface{} {
	payload := requestPayload(t, testCase)
	response, err := http.Post(testCase.url, "application/json", strings.NewReader(payload))
	require.NoError(t, err)

	responseBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.NoError(t, err)

	result := make(map[string]interface{})
	require.NoError(t, json.Unmarshal(responseBody, &result))
	return result //{"cluster":"e2etest-cluster","clusterVersion":1,"configurationVersion":1,"status":"reconcile_pending","statusUrl":"localhost:8080/v1/clusters/e2etest-cluster/configs/1/status"}

}

func requestPayload(t *testing.T, testCase *TestStruct) string {
	data, err := ioutil.ReadFile(testCase.requestFile)
	require.NoError(t, err)

	data = overrideKubeConfig(testCase.kubeconfig, data)

	return string(data)
}

func requireClusterStatus(t *testing.T, statusURL string, expected model.Status, timeout time.Duration) {
	startTime := time.Now()
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		response, err := http.Get(statusURL)
		require.NoError(t, err)

		body, _ := ioutil.ReadAll(response.Body)
		require.NoError(t, response.Body.Close())

		payload := make(map[string]interface{})
		require.NoError(t, json.Unmarshal(body, &payload))

		if payload["status"] == expected {
			return
		}
		if time.Since(startTime) >= timeout {
			t.Logf("Timeout reached: latest status of cluster '%s' was '%s' but expected was '%s'",
				clusterName, payload["status"], expected)
			break
		}
	}
	t.Fail()
}

func overrideKubeConfig(overrideKubeConfig string, data []byte) []byte {
	if overrideKubeConfig != "" {
		m := make(map[string]interface{})
		_ = json.Unmarshal(data, &m)
		m["kubeConfig"] = overrideKubeConfig
		data, _ = json.Marshal(m)
	}
	return data
}
