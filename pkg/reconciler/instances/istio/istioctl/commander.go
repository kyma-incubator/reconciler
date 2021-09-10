package istioctl

import (
	"os"
	"os/exec"
)

const (
	istioctlBinaryPathEnvKey = "ISTIOCTL_PATH"
)

//go:generate mockery -name=Commander
// Commander for istioctl binary.
type Commander interface {

	// Install wraps istioctl installation command.
	Install(istioCtlPath, istioOperatorPath, kubeconfigPath string) error
}

// DefaultCommander provides a default implementation of Commander.
type DefaultCommander struct{}

func (c *DefaultCommander) Install(istioCtlPath, istioOperatorPath, kubeconfigPath string) error {
	cmd := exec.Command(istioCtlPath, "apply", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
