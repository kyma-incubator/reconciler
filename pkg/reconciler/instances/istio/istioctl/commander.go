package istioctl

import (
	"github.com/kyma-incubator/reconciler/pkg/features"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/executor"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"os/exec"
)

// Commander for istioctl binary.
//
//go:generate mockery --name=Commander --output=mocks --case=underscore
type Commander interface {

	// Install wraps `istioctl installation` command.
	Install(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error

	// Upgrade wraps `istioctl upgrade` command.
	Upgrade(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error

	// Version wraps `istioctl version` command.
	Version(kubeconfig string, logger *zap.SugaredLogger) ([]byte, error)

	// Uninstall wraps `istioctl x uninstall` command.
	Uninstall(kubeconfig string, logger *zap.SugaredLogger) error
}

var execCommand = exec.Command

// DefaultCommander provides a default implementation of Commander.
type DefaultCommander struct {
	istioctl        Executable
	commandExecutor executor.CmdExecutor
}

func NewDefaultCommander(istioctl Executable) DefaultCommander {
	return DefaultCommander{istioctl, &executor.DefaultCmdExecutor{}}
}

func (c *DefaultCommander) Uninstall(kubeconfig string, logger *zap.SugaredLogger) error {

	kubeconfigPath, kubeconfigCf, err := file.CreateTempFileWith(kubeconfig)
	if err != nil {
		return err
	}

	defer func() {
		cleanupErr := kubeconfigCf()
		if cleanupErr != nil {
			logger.Error(cleanupErr)
		}
	}()

	return c.commandExecutor.RuntWithRetry(logger, c.istioctl.path, "x", "uninstall", "--purge", "--kubeconfig", kubeconfigPath, "--skip-confirmation")
}

func (c *DefaultCommander) Install(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error {

	kubeconfigPath, kubeconfigCf, err := file.CreateTempFileWith(kubeconfig)
	logger.Debugf("Created kubeconfig temp file on %s ", kubeconfigPath)
	if err != nil {
		return err
	}

	defer func() {
		cleanupErr := kubeconfigCf()
		if cleanupErr != nil {
			logger.Error(cleanupErr)
		}
	}()

	istioOperatorPath, istioOperatorCf, err := file.CreateTempFileWith(istioOperator)
	logger.Debugf("Created IstioOperator temp file on %s ", istioOperatorPath)
	if err != nil {
		return err
	}

	defer func() {
		cleanupErr := istioOperatorCf()
		if cleanupErr != nil {
			logger.Error(cleanupErr)
		}
	}()

	logger.Debugf("Creating executable istioctl apply command")
	err = c.commandExecutor.RuntWithRetry(logger, c.istioctl.path, "apply", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")

	if err != nil && features.Enabled(features.LogIstioOperator) {
		logger.Errorf("Got error from executing istioctl apply %v", err)
		return errors.Wrapf(err, "rendered IstioOperator yaml was: %s ", istioOperator)
	}
	return err
}

func (c *DefaultCommander) Upgrade(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error {
	return c.Install(istioOperator, kubeconfig, logger)
}

func (c *DefaultCommander) Version(kubeconfig string, logger *zap.SugaredLogger) ([]byte, error) {

	kubeconfigPath, kubeconfigCf, err := file.CreateTempFileWith(kubeconfig)
	if err != nil {
		return []byte{}, err
	}

	defer func() {
		cleanupErr := kubeconfigCf()
		if cleanupErr != nil {
			logger.Error(cleanupErr)
		}
	}()

	cmd := execCommand(c.istioctl.path, "version", "--output", "json", "--kubeconfig", kubeconfigPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return []byte{}, err
	}

	return out, nil
}
