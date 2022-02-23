package provisioning

import "github.com/kyma-incubator/reconciler/pkg/reconciler/service"

type ProvisioningAction struct {
	name       string
	kubeconfig string
}

func (a *ProvisioningAction) Run(context *service.ActionContext) error {
	if _, err := context.KubeClient.Clientset(); err != nil { //example how to retrieve native Kubernetes GO client
		context.Logger.Errorf("Failed to retrieve native Kubernetes GO client")
	}

	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, context.Task.Version)

	return nil
}
