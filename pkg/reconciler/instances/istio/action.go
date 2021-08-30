package istio

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"os/exec"
)

const (
	istioctl1_10_2 = "/bin/istioctl-1.10.2"
)

type ReconcileAction struct {
}

func (a *ReconcileAction) Run(version, profile string, config []reconciler.Configuration, context *service.ActionContext) error {
	ws, err := context.WorkspaceFactory.Get(version)
	if err != nil {
		return errors.Wrap(err,
			fmt.Sprintf("Failed to retrieve Kyma workspace '%s' in ISTIO reconcile-action", version))
	}
	fmt.Printf("Kyma sources are located here: %s\n", ws.WorkspaceDir)

	return istioctl(version)
}

//istioctl calls the istioctl command depending on the provided Kyma version
func istioctl(version string) error {
	switch version {
	case "2.0.0":
		return exec.Command(istioctl1_10_2, "version").Run()
	default:
		return exec.Command(istioctl1_10_2, "version").Run()
	}
}
