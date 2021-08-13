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
	ovverideKubeConfig string
}

const (
	EnvHostForE2ETest       = "RECONCILER_HOST_FOR_E2E_TEST"
	EnvPortForE2ETest       = "RECONCILER_PORT_FOR_E2E_TEST"
	EnvKubeConfigForE2ETest = "RECONCILER_KUBE_CONFIG_FOR_E2E_TEST"
)

//TODO afte
func TestReconciliation(t *testing.T) {
	if !test.RunExpensiveTests() {
		//return
	}
	clusterInventoryUrl := inventoryUrl()
	registerClusterUrl := clusterInventoryUrl
	getLatestConfigClusterUrl := clusterInventoryUrl + "%s/status"

	kubeConfig, _ := os.LookupEnv(EnvKubeConfigForE2ETest)
	tests := []TestStruct{
		{
			requestFile:        "request/correct_simple_request.json",
			delay:              2 * time.Minute,
			expectedStatus:     "ready",
			ovverideKubeConfig: kubeConfig,
		},
		//{
		//	requestFile: "request/correct_simple_request.json",
		//	delay: 2 * time.Minute,
		//	expectedStatus: "ready",
		//},
		//{ FIXME uncomment after https://github.com/kyma-incubator/reconciler/issues/91
		//	requestFile: "request/wrong_kubeconfig.json",
		//	delay: 2 * time.Minute,
		//	expectedStatus: "ready" ,
		//},
	}

	for _, testStruct := range tests {

		//Register cluster
		data, err := ioutil.ReadFile(testStruct.requestFile)
		if err != nil {
			t.Error(err) //Something is wrong while sending request
		}
		requestBody := string(data)
		reader := strings.NewReader(requestBody)
		request, err := http.NewRequest("POST", registerClusterUrl, reader)
		res, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Error(err) //Something is wrong while sending request
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Error(err) //Something is wrong while sending request
		}

		time.Sleep(testStruct.delay)

		// Get latest status
		url := fmt.Sprintf(getLatestConfigClusterUrl, "runtimeTest")
		request, err = http.NewRequest("GET", url, reader)
		res, err = http.DefaultClient.Do(request)
		if err != nil {
			t.Error(err) //Something is wrong while sending request
		}
		body, _ = ioutil.ReadAll(res.Body)
		m := make(map[string]interface{})
		err = json.Unmarshal(body, &m)
		if err != nil {
			t.Error(err) //Something is wrong while sending request
		}

		if m["status"] != testStruct.expectedStatus {
			t.Errorf("Failed Case:\n  request body : %s \n expectedStatus : %s \n responseBody : %s \n actualStatus : %s \n", requestBody, testStruct.expectedStatus, string(body), m["status"])
		}
	}
}

func inventoryUrl() string {
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

//t.Logf("Passed Case:\n  request body : %s \n expectedStatus : %d \n responseBody : %s \n observedStatusCode : %d \n", test.requestBody, test.expectedStatusCode, test.responseBody, test.observedStatusCode)
