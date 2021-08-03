package example

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
)

type CustomAction struct {
	name string
}

func (a *CustomAction) Run(version string, kubeClient kubernetes.Client) error {
	log, err := logger.NewLogger(true)
	if err != nil {
		return err
	}

	if _, err := kubeClient.Clientset(); err != nil { //example how to retrieve native Kubernetes GO client
		log.Errorf("Failed to retrieve native Kubernetes GO client")
	}

	log.Infof("Action '%s' executed (passed version was '%s')", a.name, version)

	return nil
}
