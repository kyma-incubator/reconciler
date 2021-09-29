package service

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type customAction struct {
	success bool
}

func (a *customAction) Run(_, _ string, _ map[string]interface{}, _ *service.ActionContext) error {
	if a.success {
		return nil
	}
	return fmt.Errorf("action failed")
}

func TestRuntimeBuilder(t *testing.T) {
	test.IntegrationTest(t)

	//register custom 'base' component reconciler for this unitest
	compRecon, err := service.NewComponentReconciler("base")
	require.NoError(t, err)
	compRecon.WithRetry(1, 1*time.Second)

	t.Run("Run local with success", func(t *testing.T) {
		compRecon.WithReconcileAction(&customAction{true})
		receivedUpdates := runLocal(t)
		require.Equal(t, receivedUpdates[len(receivedUpdates)-1].Status, reconciler.StatusSuccess)
	})

	t.Run("Run local with error", func(t *testing.T) {
		compRecon.WithReconcileAction(&customAction{false})
		receivedUpdates := runLocal(t)
		require.Equal(t, receivedUpdates[len(receivedUpdates)-1].Status, reconciler.StatusError)
	})

}

func runLocal(t *testing.T) []*reconciler.CallbackMessage {
	//create cluster entity
	inventory, err := cluster.NewInventory(db.NewTestConnection(t), true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)
	clusterState, err := inventory.CreateOrUpdate(1, &keb.Cluster{
		Kubeconfig: test.ReadKubeconfig(t),
		KymaConfig: keb.KymaConfig{
			Components: nil,
			Profile:    "",
			Version:    "1.2.3",
		},
		RuntimeID: "testCluster",
	})
	require.NoError(t, err)

	//create reconciliation repository
	reconRepo := reconciliation.NewInMemoryReconciliationRepository()
	runtimeBuilder := NewRuntimeBuilder(reconRepo, logger.NewLogger(true))

	//configure local runner
	var receivedUpdates []*reconciler.CallbackMessage
	localRunner := runtimeBuilder.RunLocal(nil, func(component string, msg *reconciler.CallbackMessage) {
		receivedUpdates = append(receivedUpdates, msg)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Second) //abort runner latest after 5 sec
	defer cancel()

	err = localRunner.Run(ctx, clusterState)
	require.NoError(t, err)
	return receivedUpdates
}
