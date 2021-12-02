package cleaner

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/cleaner/pkg/cleanup"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type CleanupAction struct {
	name string
}

func (a *CleanupAction) Run(context *service.ActionContext) error {
	if _, err := context.KubeClient.Clientset(); err != nil { //cleaner how to retrieve native Kubernetes GO client
		return err
	}

	/*
		kubeconfigPath, kubeconfigCf, fileErr := file.CreateTempFileWith(context.Task.Kubeconfig)
		if fileErr != nil {
			err = fileErr
			return
		}
		defer func() {
			err = kubeconfigCf()
		}()
		context.Logger.Infof("kubeconfig path: %s", kubeconfigPath)
	*/

	context.Logger.Infof("Action '%s' executed: passed version was '%s', passed type was %s", a.name, context.Task.Version, context.Task.Type)

	//var cliCleaner *CliCleaner
	cliCleaner, err := cleanup.NewCliCleaner(context.Task.Kubeconfig, context.Logger)
	if err != nil {
		return err
	}

	return cliCleaner.Run()
}
