package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type CustomAction struct {
	name        string
	copyFactory []CopyFactory
}

func (a *CustomAction) Run(_, _ string, _ []reconciler.Configuration, context *service.ActionContext) error {
	for _, create := range a.copyFactory {
		operation := create(context)
		err := operation.Transfer()
		if err != nil {
			return err
		}
	}

	return nil
}
