package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type CustomAction struct {
	name        string
	copyFactory []CopyFactory
}

func (a *CustomAction) Run(version, _ string, _ []reconciler.Configuration, context *service.ActionContext) error {
	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, version)

	for _, create := range a.copyFactory {
		operation := create(context)
		err := operation.Transfer()
		if err != nil {
			return err
		}
	}

	return nil
}
