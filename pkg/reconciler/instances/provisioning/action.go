package provisioning

import (
	"fmt"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/gardener"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"io/ioutil"
	restclient "k8s.io/client-go/rest"
	"os"
)

const envVarGardenerKubeconfig = "GARDENER_KUBECONFIG_PATH"

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

func newGardenerClusterConfig() (*restclient.Config, error) {

	if !file.Exists(os.Getenv(envVarGardenerKubeconfig)) {
		return nil, fmt.Errorf("kubeconfig file does not exist in path %s", envVarGardenerKubeconfig)
	}

	rawKubeconfig, err := ioutil.ReadFile(envVarGardenerKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to read Gardener Kubeconfig from path %s: %s", envVarGardenerKubeconfig, err.Error())
	}

	gardenerClusterConfig, err := gardener.Config(rawKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gardener cluster config: %s", err.Error())
	}

	return gardenerClusterConfig, nil
}
