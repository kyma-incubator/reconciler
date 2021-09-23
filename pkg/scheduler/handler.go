package scheduler

import (
	"context"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type ReconciliationHander struct {
	workerFactory WorkerFactory
	statusUpdater ClusterStatusUpdater
}

func NewReconciliationHandler(workerFactory WorkerFactory) *ReconciliationHander {
	return &ReconciliationHander{workerFactory: workerFactory}
}

func (h *ReconciliationHander) WithStatusUpdater(statusUpdater ClusterStatusUpdater) *ReconciliationHander {
	h.statusUpdater = statusUpdater
	return h
}

func (h *ReconciliationHander) Reconcile(ctx context.Context, seq *model.ReconciliationSequence, state *cluster.State, schedulingID string) error {
	//Reconcile the prerequisites
	//FIXME
	//for _, component := range seq.FirstInSequence {
	//	err := h.reconcile(ctx, component, state, schedulingID)
	//	if err != nil {
	//		return err
	//	}
	//}
	//
	////Reconcile the rest
	//g, _ := errgroup.WithContext(ctx)
	//for _, c := range seq.InParallel {
	//	component := c // https://golang.org/doc/faq#closures_and_goroutines
	//	g.Go(func() error {
	//		return h.reconcile(ctx, component, state, schedulingID)
	//	})
	//}
	//
	//return g.Wait()
	return nil
}

func (h *ReconciliationHander) reconcile(ctx context.Context, component *keb.Component, state *cluster.State, schedulingID string) error {
	worker, err := h.workerFactory.ForComponent(component.Component)
	if err != nil {
		h.statusUpdater.Update(component.Component, model.OperationStateError)
		return fmt.Errorf("failed to create a worker: %s", err)
	}

	err = worker.Reconcile(ctx, component, *state, schedulingID)
	if err != nil {
		h.statusUpdater.Update(component.Component, model.OperationStateError)
		return fmt.Errorf("failed to reconcile a component: %s: %s", component.Component, err)
	}

	h.statusUpdater.Update(component.Component, model.OperationStateDone)
	return nil
}
