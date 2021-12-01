package cleaner

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

//TODO: please implement component specific action logic here
type CleanupAction struct {
	name string
}

func (a *CleanupAction) Run(context *service.ActionContext) error {
	if _, err := context.KubeClient.Clientset(); err != nil { //cleaner how to retrieve native Kubernetes GO client
		context.Logger.Errorf("Failed to retrieve native Kubernetes GO client")
	}

	context.Logger.Infof("Action '%s' executed: passed version was '%s', passed type was %s", a.name, context.Task.Version, context.Task.Type)

	return nil
}
