package invoker

import (
	"context"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

type unittestReconcileAction struct {
	simulateError bool
}

func (a *unittestReconcileAction) Run(_ *service.ActionContext) error {
	if a.simulateError {
		return errors.New("reconciliation failed")
	}
	return nil
}

func TestLocalInvoker(t *testing.T) {
	test.IntegrationTest(t)

	t.Run("Run local reconciler: successfully finished reconciliation", func(t *testing.T) {
		reconRepo, opEntity := runLocalReconciler(t, false)
		requireOperationState(t, reconRepo, opEntity, model.OperationStateDone)
	})

	t.Run("Run local reconciler: failing reconciliation", func(t *testing.T) {
		reconRepo, opEntity := runLocalReconciler(t, true) //will lead to context timeout => op-state has to be 'error'
		requireOperationState(t, reconRepo, opEntity, model.OperationStateError)
	})

}

func runLocalReconciler(t *testing.T, simulateError bool) (reconciliation.Repository, *model.OperationEntity) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second) //stop the execution after 1 sec latest
	defer cancel()

	//register 'unittest' component reconciler
	reconAct := &unittestReconcileAction{simulateError}
	compRecon, err := service.NewComponentReconciler("unittest")
	require.NoError(t, err)
	compRecon.WithReconcileAction(reconAct)

	//init recon repository
	reconRepo := reconciliation.NewInMemoryReconciliationRepository()

	//create reconciliation entity
	reconEntity, err := reconRepo.CreateReconciliation(clusterStateMock, nil)
	require.NoError(t, err)

	//retrieve ops of reconciliation entity
	opEntities, err := reconRepo.GetOperations(reconEntity.SchedulingID)
	require.NoError(t, err)
	require.Len(t, opEntities, 2)
	opEntity := opEntities[0]

	//create callback fct for receiving reconciler feedbacks
	var callbacks []*reconciler.CallbackMessage
	callbackFct := func(component string, msg *reconciler.CallbackMessage) {
		callbacks = append(callbacks, msg)
	}

	clusterStateMock.Cluster.Kubeconfig = test.ReadKubeconfig(t)

	invoker := NewLocalReconcilerInvoker(reconRepo, callbackFct, logger.NewLogger(true))
	err = invoker.Invoke(ctx, &Params{
		ComponentToReconcile: &keb.Component{
			Component:     "unittest", //will trigger the 'unittest' component reconciler created above
			Configuration: nil,
			Namespace:     "kyma-system",
			Version:       "1.2.3",
		},
		ComponentsReady: nil,
		ClusterState:    clusterStateMock,
		SchedulingID:    opEntity.SchedulingID,
		CorrelationID:   opEntity.CorrelationID,
	})

	if simulateError {
		require.Error(t, err)
		require.IsType(t, err, context.DeadlineExceeded)
	} else {
		require.NoError(t, err)
	}

	time.Sleep(500 * time.Millisecond) //give the component reconciler some time to finish its work

	return reconRepo, opEntity
}
