package service

import (
	"context"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/test"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/stretchr/testify/require"
)

type cleanup struct {
	reconciler *ComponentReconciler
	kubeClient kubernetes.Client
}

func NewTestCleanup(reconciler *ComponentReconciler, kubeClient kubernetes.Client) *cleanup {
	return &cleanup{
		reconciler: reconciler,
		kubeClient: kubeClient,
	}
}

func (c *cleanup) RemoveKymaComponent(t *testing.T, version, component, namespace string) {
	t.Logf("Cleanup of component '%s' (version: %s, namespace: %s) started", component, version, namespace)

	//render manifest
	chartProv, err := c.reconciler.newChartProvider()
	require.NoError(t, err)

	comp := chart.NewComponentBuilder(version, component).
		WithNamespace(namespace).
		WithConfiguration(test.NewGlobalComponentConfiguration()).
		Build()

	manifest, err := chartProv.RenderManifest(comp)
	require.NoError(t, err)

	//delete resources in manifest
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute) //deletion has to happen within 1 min
	defer cancel()
	_, err = c.kubeClient.Delete(ctx, manifest.Manifest, namespace) //blocking call until all watchable resources were removed
	require.NoError(t, err)

	t.Logf("Cleanup of component '%s' finished", component)
}
