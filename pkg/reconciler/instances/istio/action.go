package istio

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
)

type InstallAction struct {
}

func (a *InstallAction) Run(version string, kubeClient kubernetes.Client) error {
	return nil
}
