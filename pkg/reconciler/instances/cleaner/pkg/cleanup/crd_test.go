package cleanup

import (
	"context"
	"testing"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCleanupFindAllCRDs(t *testing.T) {
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

func TestEnsureNoneConversionWebhook(t *testing.T) {
	test.IntegrationTest(t)
	logger := log.NewLogger(true)
	kubeconfig := test.ReadKubeconfig(t)
	cliCleaner, err := NewCliCleaner(kubeconfig, nil, logger, false, nil)
	require.NoError(t, err)

	// deploy crd
	crdManifest := test.ReadManifest(t, "test-crd-with-conversion.yaml")
	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), logger, nil)
	require.NoError(t, err)
	_, err = kubeClient.Deploy(context.TODO(), crdManifest, "default")
	require.NoError(t, err)

	crdef := schema.GroupVersionResource{
		Group:    "foocr.kyma",
		Version:  "v1",
		Resource: "tobecleaneds",
	}

	// ensure no conversion webhook
	err = cliCleaner.ensureNoConversionWebhooksFor(crdef)
	require.NoError(t, err)

	// verify crd
	crdToVerify, err := kubeClient.Get("CustomResourceDefinition", "tobecleaneds.foocr.kyma", "default")
	require.NoError(t, err)
	require.NotEmpty(t, crdToVerify)

	conversionStrategy, ok, err := unstructured.NestedString(crdToVerify.Object, "spec", "conversion", "strategy")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, conversionStrategy, "None")

	// delete crd
	_, err = kubeClient.Delete(context.TODO(), crdManifest, "default")
	require.NoError(t, err)

}
