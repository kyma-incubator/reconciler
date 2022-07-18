package cleanup

import (
	"context"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func TestCleanupFindAllCRDs(t *testing.T) {
	t.Skip("skip")
	test.IntegrationTest(t)
	logger := log.NewLogger(true)
	kubeconfig := test.ReadKubeconfig(t)
	cliCleaner, err := NewCliCleaner(kubeconfig, nil, logger, false, nil)
	require.NoError(t, err)

	// deploy crd
	crdManifest := test.ReadManifest(t, "unittest-crd.yaml")
	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger, nil)
	require.NoError(t, err)
	_, err = kubeClient.Deploy(context.TODO(), crdManifest, "default")
	require.NoError(t, err)

	// check crd exits
	crds, err := cliCleaner.findAllCRDsInCluster()
	require.NoError(t, err)
	require.Contains(t, crds, schema.GroupVersionResource{
		Group:    "foocr.kyma",
		Version:  "v1",
		Resource: "tobecleaneds",
	})

	// delete crd
	_, err = kubeClient.Delete(context.TODO(), crdManifest, "default")
	require.NoError(t, err)

}
