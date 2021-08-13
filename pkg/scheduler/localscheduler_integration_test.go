package scheduler

import (
	"context"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestStuff(t *testing.T) {
	t.Skip()

	kubeconfig := test.ReadKubeconfig(t)

	l, _ := logger.NewLogger(false)
	_, err := service.NewComponentReconciler("cluster-essentials")
	if err != nil {
		l.Fatalf("Could not create '%s' component reconciler: %s", "cluster-essentials", err)
	}
	_, err = service.NewComponentReconciler("istio")
	if err != nil {
		l.Fatalf("Could not create '%s' component reconciler: %s", "istio", err)
	}

	operationsRegistry := NewDefaultOperationsRegistry()

	workerFactory, err := NewLocalWorkerFactory(
		&cluster.MockInventory{},
		operationsRegistry,
		true)
	require.NoError(t, err)

	ls := LocalScheduler{
		clusterState: cluster.State{
			Cluster: &model.ClusterEntity{
				Kubeconfig: kubeconfig,
			},
			Configuration: &model.ClusterConfigurationEntity{
				Contract:    1,
				KymaVersion: "main",
				KymaProfile: "evaluation",
				Components: fixUgliness([]keb.Components{
					{
						Component: "cluster-essentials",
						Namespace: "kyma-system",
					},
					{
						Component: "istio",
						Namespace: "istio-system",
					},
				}),
			},
		},
		workerFactory: workerFactory,
		logger:        l,
	}

	err = ls.Run(context.Background())
	require.NoError(t, err)
}
