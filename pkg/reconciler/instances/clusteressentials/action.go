package clusteressentials

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type CustomAction struct {
	name string
}

func (a *CustomAction) Run(context *service.ActionContext) error {
	context.Logger.Debugf("Action '%s' executed (passed version was '%s')", a.name, context.Task.Version)
	return service.NewInstall(context.Logger).Invoke(context.Context, context.ChartProvider, context.Task, context.KubeClient)
}
