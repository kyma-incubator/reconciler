package reconcileaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/step"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	actionName = "reconcile-action"
)

// ReconcileAction represents an action that runs in the Eventing reconciliation phase.
// It is composed of ReconcileAction steps.
type ReconcileAction struct {
	steps step.Steps
}

// New returns a new ReconcileAction instance.
func New() *ReconcileAction {
	return &ReconcileAction{
		steps: step.Steps{
			// add ReconcileAction steps here
		},
	}
}

// Run reconciler ReconcileAction logic for Eventing. It executes the ReconcileAction steps in order
// and returns a non-nil error if any step was unsuccessful.
func (a *ReconcileAction) Run(context *service.ActionContext) (err error) {
	// prepare logger
	logger := log.ContextLogger(context, log.WithAction(actionName))

	// execute steps
	for _, s := range a.steps {
		if err := s.Execute(context, logger); err != nil {
			return err
		}
	}

	return nil
}
