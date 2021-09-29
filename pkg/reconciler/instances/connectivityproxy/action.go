package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type CustomAction struct {
	name        string
	copyFactory []CopyFactory
}

func (a *CustomAction) Run(context *service.ActionContext) error {

	clientset, err := context.KubeClient.Clientset()
	if err != nil {
		return err
	}

	inClusterClientSet, err := kubeclient.NewInClusterClientSet(context.Logger)
	if err != nil {
		return err
	}

	for _, create := range a.copyFactory {
		operation := create(context.Model.Configuration, inClusterClientSet, clientset)
		err := operation.Transfer()
		if err != nil {
			return err
		}
	}

	return nil
}
