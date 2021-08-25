package e2e

import (
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type TestStruct struct {
	requestFile        string
	delay              time.Duration
	expectedStatus     string
	overrideKubeConfig string
}

const (
	EnvHostForE2ETest       = "RECONCILER_HOST_FOR_E2E_TEST"
	EnvPortForE2ETest       = "RECONCILER_PORT_FOR_E2E_TEST"
	EnvKubeConfigForE2ETest = "RECONCILER_KUBE_CONFIG_FOR_E2E_TEST"
)

func TestReconciliation(t *testing.T) {
	test.IntegrationTest(t)

	if true { // FIXME remove after created dedicated pipeline
		return
	}

	clusterInventoryURL := inventoryURL()
	registerClusterURL := clusterInventoryURL
	getLatestConfigClusterURL := clusterInventoryURL + "%s/status"

	kubeConfig, _ := os.LookupEnv(EnvKubeConfigForE2ETest)
	tests := []TestStruct{
		{
			requestFile:        "request/correct_simple_request.json",
			delay:              2 * time.Minute,
			expectedStatus:     "ready",
			overrideKubeConfig: kubeConfig,
		},
		//{ FIXME uncomment after https://github.com/kyma-incubator/reconciler/issues/91
		//	requestFile: "request/wrong_kubeconfig.json",
		//	delay: 2 * time.Minute,
		//	expectedStatus: "ready" ,
		//},
	}

	for _, testCase := range tests {

		//Register cluster
		data, err := ioutil.ReadFile(testCase.requestFile)
		if err != nil {
			t.Error(err) //Something is wrong while sending request
		}

		data = overrideKubeConfig(testCase.overrideKubeConfig, data)
		requestBody := string(data)
		reader := strings.NewReader(requestBody)
		request, err := http.NewRequest("POST", registerClusterURL, reader)
		if err != nil {
			t.Error(err)
		}
		res, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Error(err)
		}
		registerClusterResponse, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Error(err)
		}

		time.Sleep(testCase.delay)

		// Get latest status
		url := fmt.Sprintf(getLatestConfigClusterURL, "runtimeTest")
		request, err = http.NewRequest("GET", url, reader)
		if err != nil {
			t.Error(err)
		}
		res, err = http.DefaultClient.Do(request)
		if err != nil {
			t.Error(err) //Something is wrong while sending request
		}
		body, _ := ioutil.ReadAll(res.Body)
		m := make(map[string]interface{})
		err = json.Unmarshal(body, &m)
		if err != nil {
			t.Error(err) //Something is wrong while sending request
		}

		if m["status"] != testCase.expectedStatus {
			t.Errorf("Failed Case:\n  request body : %s \n expectedStatus : %s \n responseBody : %s \n actualStatus : %s \n", requestBody, testCase.expectedStatus, string(registerClusterResponse), m["status"])
		}
	}
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

func inventoryURL() string {
	host, ok := os.LookupEnv(EnvHostForE2ETest)
	if !ok {
		host = "localhost"
	}
	port, ok := os.LookupEnv(EnvPortForE2ETest)
	if !ok {
		port = "8080"
	}
	return fmt.Sprintf("http://%s:%s/v1/clusters/", host, port)
}
