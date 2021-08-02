package cmd

import (
	"github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type Options struct {
	*reconciler.Options
	Name            string
	Dependencies    []string
	PreAction       service.Action
	ReconcileAction service.Action
	PostAction      service.Action
}

func NewOptions(o *reconciler.Options) *Options {
	//CONFIGURE COMPONENT-RECONCILER SPECIFIC VALUES HERE:
	return &Options{
		o,
		"example",  //name
		[]string{}, //dependencies (components which have to be installed before this component reconciler can run)
		nil,        //pre-reconcile action (logic to execute before reconciliation happened)
		nil,        //reconcile action (custom reconciliation logic)
		nil,        //post-reconcile action (logic to execute after reconciliation happened)
	}
}
