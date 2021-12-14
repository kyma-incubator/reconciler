package cleaner

import (
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/cleaner/pkg/cleanup"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type CleanupAction struct {
	name string
}

func (a *CleanupAction) Run(context *service.ActionContext) error {
	if context.Task.Type != model.OperationTypeDelete {
		context.Logger.Infof("Skipping execution. This reconciler only supports 'delete' task type, but was invoked with '%s' task type", context.Task.Type)
		return nil
	}

	if _, err := context.KubeClient.Clientset(); err != nil { //cleaner how to retrieve native Kubernetes GO client
		return err
	}

	context.Logger.Infof("Action '%s' executed: passed version was '%s', passed type was %s", a.name, context.Task.Version, context.Task.Type)

	namespaces := []string{"kyma-system", "kyma-integration"}
	cliCleaner, err := cleanup.NewCliCleaner(context.Task.Kubeconfig, namespaces, context.Logger)
	if err != nil {
		return err
	}

	return cliCleaner.Run()
}
