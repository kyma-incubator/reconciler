package example

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

//TODO: please implement component specific action logic here
type CustomAction struct {
	name string
}

func (a *CustomAction) Run(context *service.ActionContext) error {
	if _, err := context.KubeClient.Clientset(); err != nil { //example how to retrieve native Kubernetes GO client
		context.Logger.Errorf("Failed to retrieve native Kubernetes GO client")
	}

	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, context.Task.Version)

	return nil
}
