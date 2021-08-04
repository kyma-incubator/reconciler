package busola_migrator_test

import (
	busola_migrator "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/busola-migrator"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"
)

func TestPreInstall(t *testing.T) {
	//GIVEN
	kubeconfig, err := ioutil.ReadFile("/Users/i515376/.kube/config")
	//kubeconfig, err := ioutil.ReadFile("/Users/i515376/gardener-k8s-config.yaml")
	require.NoError(t, err)
	kubeClient, err := kubernetes.NewKubernetesClient(string(kubeconfig), true)
	require.NoError(t, err)
	p := busola_migrator.NewVirtualServicePreInstallPatch("dex-virtualservice", "kyma-system", "console-web", "kyma-system", "-old")

	clientSet, err := kubeClient.Clientset()
	require.NoError(t, err)

	//WHEN
	err = p.Run("", clientSet)

	//THE
	require.NoError(t, err)
}

func TestNewVirtualServicePreInstallPatch(t *testing.T) {
}