package preaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/step"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	actionName = "pre-action"
)

// PreAction represents an action that runs before the Eventing reconciliation phase.
// It is composed of PreAction steps.
type PreAction struct {
	steps step.Steps
}

// New returns a new PreAction instance.
func New() *PreAction {
	return &PreAction{
		steps: step.Steps{
			new(migrateEventTypePrefixConfigStep),
		},
	}
}

// Run reconciler PreAction logic for Eventing. It executes the PreAction steps in order
// and returns a non-nil error if any step was unsuccessful.
func (a *PreAction) Run(context *service.ActionContext) (err error) {
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
