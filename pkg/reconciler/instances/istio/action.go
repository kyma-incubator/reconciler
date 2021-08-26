package istio

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
)

const (
	istioctl1_10_2 = "istioctl-1.10.2"
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
	var out bytes.Buffer
	cmd := exec.Command(istioctl1_10_2, "install", "-y", "--set", "profile=demo")
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Istio installation failed: %s\nistioctl output:%s\n", err, out.String())
	}
	return err
}
