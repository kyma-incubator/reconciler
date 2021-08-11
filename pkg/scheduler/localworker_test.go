package scheduler

import (
	"testing"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestStuff(t *testing.T) {
	kubeconfig := test.ReadKubeconfig(t)

	component := "cluster-essentials"

	logger, _ := logger.NewLogger(false)
	logger.Debugf("Initializing component reconciler '%s'", component)
	_, err := service.NewComponentReconciler(component)
	if err != nil {
		logger.Fatalf("Could not create '%s' component reconciler: %s", component, err)
	}

	worker, err := NewWorker(
		&reconciler.ComponentReconciler{},
		&cluster.MockInventory{},
		NewDefaultOperationsRegistry(),
		&LocalReconcilerInvoker{logger: logger},
		true)
	require.NoError(t, err)

	worker.Reconcile(
		&keb.Components{
			Component: component,
			Namespace: "kyma-system",
		},
		cluster.State{
			Cluster: &model.ClusterEntity{
				Kubeconfig: kubeconfig,
			},
			Configuration: &model.ClusterConfigurationEntity{
				KymaVersion: "main",
				KymaProfile: "evaluation",
			},
		},
		uuid.NewString())
	require.NoError(t, err)
}
