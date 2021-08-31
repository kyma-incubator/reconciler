package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

type cleanup struct {
	reconciler *ComponentReconciler
	kubeClient kubernetes.Client
}

func (c *cleanup) removeKymaComponent(t *testing.T, version, component, namespace string) {
	t.Logf("Cleanup of component '%s' (version: %s, namespace: %s) started", component, version, namespace)

	//render manifest
	chartProv, err := c.reconciler.newChartProvider(&reconciler.Repo{})
	require.NoError(t, err)

	compSet := chart.NewComponentSet(

		test.ReadKubeconfig(t),
		version,
		namespace,
		[]*chart.Component{
			chart.NewComponent(component, namespace, nil),
		},
	)

	manifests, err := chartProv.Manifests(compSet, false, &chart.Options{})
	require.NoError(t, err)
	require.Len(t, manifests, 1)

	//delete resources in manifest
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute) //deletion has to happen within 1 min
	defer cancel()
	_, err = c.kubeClient.Delete(ctx, manifests[0].Manifest, namespace) //blocking call until all watchable resources were removed
	require.NoError(t, err)

	t.Logf("Cleanup of component '%s' finished", component)
}
