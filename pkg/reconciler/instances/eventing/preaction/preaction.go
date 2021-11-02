package preaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/step"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	actionName = "pre-action"
)

// preAction represents an action that runs before the Eventing reconciliation phase.
// It is composed of preAction steps.
type preAction struct {
	steps step.Steps
}

// New returns a new preAction instance.
func New() *preAction {
	return &preAction{
		steps: step.Steps{
			new(migrateEventTypePrefixConfigStep),
		},
	}
}

// Run reconciler preAction logic for Eventing. It executes the preAction steps in order
// and returns a non-nil error if any step was unsuccessful.
func (a *preAction) Run(context *service.ActionContext) (err error) {
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
