package action

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

// Action represents an action that runs along with the Eventing reconciler. It is composed of multiple Steps.
type Action struct {
	name  string
	steps Steps
}

// New returns a new Action instance.
func New(name string, steps Steps) *Action {
	return &Action{name: name, steps: steps}
}

// Run reconciler Action logic for Eventing. It executes the Action steps in order
// and returns a non-nil error if any step was unsuccessful.
func (a *Action) Run(context *service.ActionContext) (err error) {
	// prepare logger
	logger := log.ContextLogger(context, log.WithAction(a.name))

	// execute steps
	for _, s := range a.steps {
		if err := s.Execute(context, logger); err != nil {
			return err
		}
	}

	return nil
}
