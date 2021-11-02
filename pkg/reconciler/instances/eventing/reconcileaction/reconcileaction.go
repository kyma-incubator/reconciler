package reconcileaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/step"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	actionName = "reconcile-action"
)

// reconcileAction represents an action that runs in the Eventing reconciliation phase.
// It is composed of reconcileAction steps.
type reconcileAction struct {
	steps step.Steps
}

// New returns a new reconcileAction instance.
func New() *reconcileAction {
	return &reconcileAction{
		steps: step.Steps{
			// TODO: add reconcileAction steps here
		},
	}
}

// Run reconciler reconcileAction logic for Eventing. It executes the reconcileAction steps in order
// and returns a non-nil error if any step was unsuccessful.
func (a *reconcileAction) Run(context *service.ActionContext) (err error) {
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
