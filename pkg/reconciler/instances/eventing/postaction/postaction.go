package postaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/step"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	actionName = "post-action"
)

// PostAction represents an action that runs after the Eventing reconciliation phase.
// It is composed of PostAction steps.
type PostAction struct {
	steps step.Steps
}

// New returns a new PostAction instance.
func New() *PostAction {
	return &PostAction{
		steps: step.Steps{
			// add PostAction steps here
		},
	}
}

// Run reconciler PostAction logic for Eventing. It executes the PostAction steps in order
// and returns a non-nil error if any step was unsuccessful.
func (a *PostAction) Run(context *service.ActionContext) (err error) {
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
