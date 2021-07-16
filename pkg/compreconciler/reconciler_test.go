package compreconciler

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"testing"
	"time"

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
	recon := NewComponentReconciler(chartProvider)

	t.Run("Verify fluent interface", func(t *testing.T) {
		preAct := &DummyAction{
			"123",
		}
		act := &DummyAction{
			"123",
		}
		postAct := &DummyAction{
			"123",
		}
		recon.Configure(987*time.Second, 123, 321*time.Second).
			Debug().
			WithPreInstallAction(preAct).
			WithInstallAction(act).
			WithPostInstallAction(postAct).
			WithServerConfiguration(9999, "sslCrtFile", "sslKeyFile")

		require.Equal(t, &ComponentReconciler{
			preInstallAction:  preAct,
			installAction:     act,
			postInstallAction: postAct,
			debug:             true,
			updateInterval:    987 * time.Second,
			maxRetries:        123,
			retryDelay:        321 * time.Second,
			chartProvider:     chartProvider,
			serverOpts: serverOpts{
				port:       9999,
				sslCrtFile: "sslCrtFile",
				sslKeyFile: "sslKeyFile",
			},
		}, recon)
	})

}
