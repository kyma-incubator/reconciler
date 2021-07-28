package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"

	"github.com/kyma-incubator/reconciler/pkg/chart"
	"github.com/kyma-incubator/reconciler/pkg/workspace"
)

type DummyAction struct {
	receivedVersion string
}

func (da *DummyAction) Run(version string, kubeClient *kubernetes.Clientset) error {
	if kubeClient != nil {
		return fmt.Errorf("kubeClient is not expected in this test case")
	}
	da.receivedVersion = version
	return nil
}

func TestReconciler(t *testing.T) {
	chartProvider := newChartProvider(t)

	t.Run("Verify fluent configuration interface", func(t *testing.T) {
		recon := NewComponentReconciler(chartProvider)

		preAct := &DummyAction{
			"123",
		}
		act := &DummyAction{
			"123",
		}
		postAct := &DummyAction{
			"123",
		}
		recon.WithRetry(111, 222*time.Second).
			WithDependencies("a", "b", "c").
			Debug().
			WithPreInstallAction(preAct).
			WithInstallAction(act).
			WithPostInstallAction(postAct).
			WithServerConfig(9999, "sslCrtFile", "sslKeyFile").
			WithStatusUpdaterConfig(333*time.Second, 444, 555*time.Second).
			WithProgressTrackerConfig(666*time.Second, 777*time.Second).
			WithWorkers(888, 999*time.Second)

		require.Equal(t, &ComponentReconciler{
			maxRetries:        111,
			retryDelay:        222 * time.Second,
			debug:             true,
			preInstallAction:  preAct,
			installAction:     act,
			postInstallAction: postAct,
			serverConfig: serverConfig{
				port:       9999,
				sslCrtFile: "sslCrtFile",
				sslKeyFile: "sslKeyFile",
			},
			statusUpdaterConfig: statusUpdaterConfig{
				interval:   333 * time.Second,
				maxRetries: 444,
				retryDelay: 555 * time.Second,
			},
			progressTrackerConfig: progressTrackerConfig{
				interval: 666 * time.Second,
				timeout:  777 * time.Second,
			},
			timeout:       999 * time.Second,
			workers:       888,
			chartProvider: chartProvider,
			dependencies:  []string{"a", "b", "c"},
		}, recon)
	})

	t.Run("Filter missing component dependencies", func(t *testing.T) {
		recon := NewComponentReconciler(chartProvider).WithDependencies("a", "b")
		require.ElementsMatch(t, []string{"a", "b"}, recon.dependenciesMissing(&reconciler.Reconciliation{
			ComponentsReady: []string{"x", "y", "z"},
		}))
		require.ElementsMatch(t, []string{"b"}, recon.dependenciesMissing(&reconciler.Reconciliation{
			ComponentsReady: []string{"a", "y", "z"},
		}))
		require.ElementsMatch(t, []string{}, recon.dependenciesMissing(&reconciler.Reconciliation{
			ComponentsReady: []string{"a", "b", "z"},
		}))
	})
}

func TestReconcilerEnd2End(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	t.Run("Missing dependencies", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			cancel()
			time.Sleep(1 * time.Second) //give some time for graceful shutdown
		}()

		go func() {
			recon := NewComponentReconciler(newChartProvider(t)).
				WithWorkers(2, 30*time.Second).
				WithServerConfig(9999, "", "").
				WithDependencies("abc", "xyz").
				Debug()
			err := recon.StartRemote(ctx)
			require.NoError(t, err)
		}()

		//send request with which does not include all required dependencies
		body := post(t, "http://localhost:9999/v1/run", reconciler.Reconciliation{
			ComponentsReady: []string{"abc", "def"},
			Component:       "unittest-component",
			Namespace:       "unittest",
			Version:         "1.2.3",
			Profile:         "unittest",
			Configuration:   nil,
			Kubeconfig:      "",
			CallbackURL:     "https://httpbin.org/post",
			InstallCRD:      false,
		}, http.StatusPreconditionRequired)

		//convert body to HTTP response model
		t.Logf("Body received: %s", string(body))
		resp := &reconciler.HttpMissingDependenciesResponse{}
		require.NoError(t, json.Unmarshal(body, resp))
		require.Equal(t, []string{"abc", "xyz"}, resp.Dependencies.Required)
		require.Equal(t, []string{"xyz"}, resp.Dependencies.Missing)
	})
}

func post(t *testing.T, url string, payload interface{}, expectedHttpCode int) []byte {
	jsonPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(url, "application/json",
		bytes.NewBuffer(jsonPayload))
	require.NoError(t, err)
	require.Equal(t, expectedHttpCode, resp.StatusCode)

	//read body
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	return body
}

func newChartProvider(t *testing.T) *chart.Provider {
	chartProvider, err := chart.NewProvider(&workspace.Factory{
		StorageDir: "./test",
	}, true)
	require.NoError(t, err)
	return chartProvider
}
