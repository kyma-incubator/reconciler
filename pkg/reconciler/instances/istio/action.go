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
	istioctl1_10_2 = "/bin/istioctl-1.10.2"
)

type ReconcileAction struct {
	istioctlPath string
}

func (a *ReconcileAction) Run(version, profile string, config []reconciler.Configuration, context *service.ActionContext) error {
	ws, err := context.WorkspaceFactory.Get(version)

	if err != nil {
		return errors.Wrap(err,
			fmt.Sprintf("Failed to retrieve Kyma workspace '%s' in ISTIO reconcile-action", version))
	}
	context.Logger.Infof("Kyma sources are located here: %s\n", ws.WorkspaceDir)
	cmd := exec.Command(a.istioctlPath, "install", "-y", "--set", "profile=demo") // #nosec G204

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err,
			fmt.Sprintf("Istio installation failed: %s\nistioctl output:%s\n", err, out.String()))
	}
	return err
}
