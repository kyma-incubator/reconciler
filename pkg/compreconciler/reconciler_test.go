package compreconciler

import (
	"fmt"
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
	chartProvider, err := chart.NewProvider(&workspace.Factory{
		StorageDir: "./test",
	}, true)
	require.NoError(t, err)

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
		}, recon)
	})

	//t.Run("Verify worker pool setup", func(t *testing.T) {
	//	recon := NewComponentReconciler(chartProvider)
	//	err := recon.StartRemote(context.Background())
	//	require.NoError(t, err)
	//})
}
