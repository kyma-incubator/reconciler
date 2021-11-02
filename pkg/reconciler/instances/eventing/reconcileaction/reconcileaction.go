package reconcileaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/action"
)

const (
	actionName = "reconcile-action"
)

// New returns a new Action instance configured to run in reconciliation phase.
func New() *action.Action {
	return action.New(actionName, action.Steps{
		// add ReconcileAction steps here
	})
}
