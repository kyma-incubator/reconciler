package istioctl

import (
	"bufio"
	"github.com/kyma-incubator/reconciler/pkg/features"
	"github.com/pkg/errors"
	"io"
	"os/exec"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"go.uber.org/zap"
)

//go:generate mockery --name=Commander --outpkg=mocks --case=underscore
// Commander for istioctl binary.
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
	istioctl Executable
}

func NewDefaultCommander(istioctl Executable) DefaultCommander {
	return DefaultCommander{istioctl}
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

	cmd := execCommand(c.istioctl.path, "x", "uninstall", "--purge", "--kubeconfig", kubeconfigPath, "--skip-confirmation")

	return c.execute(cmd, logger)
}

func (c *DefaultCommander) Install(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error {

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

	istioOperatorPath, istioOperatorCf, err := file.CreateTempFileWith(istioOperator)
	if err != nil {
		return err
	}

	defer func() {
		cleanupErr := istioOperatorCf()
		if cleanupErr != nil {
			logger.Error(cleanupErr)
		}
	}()

	cmd := execCommand(c.istioctl.path, "apply", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")

	err = c.execute(cmd, logger)
	if features.LogIstioOperator() {
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

func (c *DefaultCommander) execute(cmd *exec.Cmd, logger *zap.SugaredLogger) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// cmd.Wait() should be called only after we finish reading from stdout and stderr
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		bufferAndLog(stdout, logger)
	}()
	go func() {
		defer wg.Done()
		bufferAndLog(stderr, logger)
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func bufferAndLog(r io.Reader, logger *zap.SugaredLogger) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Info(scanner.Text())
	}
}
