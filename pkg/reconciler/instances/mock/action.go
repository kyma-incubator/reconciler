package mock

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"time"
)

const (
	sleepTime = 1000
)

// CustomAction for mock component reconcilation.
type CustomAction struct {
	name string
}

func (a *CustomAction) Run(context *service.ActionContext) error {
	context.Logger.Infof("Starting reconcilation of component %s", context.Task.Component)
	context.Logger.Infof("Sleeping for %d...", sleepTime)
	time.Sleep(sleepTime)
	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, context.Task.Version)
	context.Logger.Infof("Finished reconcilation of component %s", context.Task.Component)

	return nil
}
