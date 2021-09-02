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
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	serverPort = 8080
)

type TestStruct struct {
	url              string
	requestFile      string
	kubeconfig       string
	expectedHTTPCode int
	verifier         func(t *testing.T, response interface{})
}

func TestReconciliation(t *testing.T) {
	test.IntegrationTest(t)

	ctx := context.Background()
	defer ctx.Done()

	startMothershipReconciler(t, ctx)

	tests := []*TestStruct{
		{
			url:              fmt.Sprintf("http://localhost:%d/v1/clusters", serverPort),
			requestFile:      filepath.Join("test", "request", "create_cluster.json"),
			kubeconfig:       test.ReadKubeconfig(t),
			expectedHTTPCode: 200,
			verifier: func(t *testing.T, response interface{}) {
				require.IsType(t, response, &keb.HTTPClusterResponse{})
				respModel := response.(*keb.HTTPClusterResponse)
				require.Equal(t, keb.ClusterStatusPending, respModel.Status)
				_, err := url.Parse(respModel.StatusURL)
				require.NoError(t, err)
			},
		},
		{
			url:              fmt.Sprintf("http://localhost:%d/v1/clusters", serverPort),
			requestFile:      filepath.Join("test", "request", "create_cluster_invalid_kubeconfig.json"),
			expectedHTTPCode: 400,
			verifier: func(t *testing.T, response interface{}) {
				require.IsType(t, response, &keb.HTTPErrorResponse{})
				errModel := response.(*keb.HTTPErrorResponse)
				require.NotEmpty(t, errModel.Error)
			},
		},
	}

	for _, testCase := range tests {
		resp := sendRequest(t, testCase)
		if testCase.verifier != nil {
			testCase.verifier(t, resp)
		}
	}
}

func startMothershipReconciler(t *testing.T, ctx context.Context) {
	go func(ctx context.Context) {
		o := NewOptions(cli.NewTestOptions(t))
		o.Port = serverPort
		o.ReconcilersCfgPath = filepath.Join("test", "component-reconcilers.json")
		o.WatchInterval = 1 * time.Second
		o.Verbose = true

		t.Log("Starting mothership reconciler")
		require.NoError(t, Run(ctx, o))
	}(ctx)

	waitForTCPSocket(t, "127.0.0.1", serverPort, 8*time.Second)
}

func waitForTCPSocket(t *testing.T, host string, port int, timeout time.Duration) {
	check := time.Tick(1 * time.Second)
	destAddr := fmt.Sprintf("%s:%d", host, port)
	for {
		select {
		case <-check:
			_, err := net.Dial("tcp", destAddr)
			if err == nil {
				return
			}
		case <-time.After(timeout):
			t.Logf("Timeout reached: could not open TCP connection to '%s' within %.1f seconds",
				destAddr, timeout.Seconds())
			t.Fail()
		}
	}
}

func sendRequest(t *testing.T, testCase *TestStruct) interface{} {
	payload := readPayload(t, testCase)
	response, err := http.Post(testCase.url, "application/json", strings.NewReader(payload))
	require.NoError(t, err)

	require.Equal(t, testCase.expectedHTTPCode, response.StatusCode, "Returned HTTP response code was unexpected")

	responseBody, err := ioutil.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.NoError(t, err)

	var result interface{}
	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		result = &keb.HTTPClusterResponse{}
	} else {
		result = &keb.HTTPErrorResponse{}
	}

	require.NoError(t, json.Unmarshal(responseBody, result))
	return result
}

func readPayload(t *testing.T, testCase *TestStruct) string {
	data, err := ioutil.ReadFile(testCase.requestFile)
	require.NoError(t, err)
	return string(overrideKubeConfig(t, data, testCase.kubeconfig))
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
