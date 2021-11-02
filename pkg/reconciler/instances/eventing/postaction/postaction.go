package postaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/step"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	actionName = "post-action"
)

// postAction represents an action that runs after the Eventing reconciliation phase.
// It is composed of postAction steps.
type postAction struct {
	steps step.Steps
}

// New returns a new postAction instance.
func New() *postAction {
	return &postAction{
		steps: step.Steps{
			// TODO: add postAction steps here
		},
	}
}

// Run reconciler postAction logic for Eventing. It executes the postAction steps in order
// and returns a non-nil error if any step was unsuccessful.
func (a *postAction) Run(context *service.ActionContext) (err error) {
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
