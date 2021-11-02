package postaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/action"
)

const (
	actionName = "post-action"
)

// New returns a new Action instance configured to run post reconciliation phase.
func New() *action.Action {
	return action.New(actionName, action.Steps{
		// add PostAction steps here
	})
}
