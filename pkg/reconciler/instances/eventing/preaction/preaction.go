package preaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/action"
)

const (
	actionName = "pre-action"
)

// New returns a new Action instance configured to run pre reconciliation phase.
func New() *action.Action {
	return action.New(actionName, action.Steps{
		// add PreAction steps here
		new(migrateEventTypePrefixConfigStep),
	})
}
