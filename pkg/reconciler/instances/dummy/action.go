package dummy

import (
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type CustomAction struct {
	name string
}

func (a *CustomAction) Run(version, profile string, config []reconciler.Configuration, context *service.ActionContext) error {
	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, version)
	st := 1 * time.Minute
	context.Logger.Infof("Going to sleep for %s", st)
	time.Sleep(st)
	context.Logger.Info("Done!")
	return nil
}
