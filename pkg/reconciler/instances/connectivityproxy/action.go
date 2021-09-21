package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
)

type CustomAction struct {
	name        string
	copyFactory []CopyFactory
}

func (a *CustomAction) Run(version, profile string, configuration map[string]interface{}, context *service.ActionContext) error {

	clientset, err := context.KubeClient.Clientset()
	if err != nil {
		return err
	}

	inClusterClientSet, err := kubeclient.NewInClusterClientSet(zap.NewNop().Sugar())
	if err != nil {
		return err
	}

	for _, create := range a.copyFactory {
		operation := create(configuration, inClusterClientSet, clientset)
		err := operation.Transfer()
		if err != nil {
			return err
		}
	}

	return nil
}
