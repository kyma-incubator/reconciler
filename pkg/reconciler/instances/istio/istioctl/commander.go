package istioctl

import (
	"os/exec"

	"bufio"
	"io"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"go.uber.org/zap"
)

//go:generate mockery --name=Commander --outpkg=mock --case=underscore
// Commander for istioctl binary.
type Commander interface {

	// Install wraps istioctl installation command.
	Install(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error

	// Upgrade wraps istioctl upgrade command.
	Upgrade(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error

	// Version wraps istioctl version command.
	Version(kubeconfig string, logger *zap.SugaredLogger) ([]byte, error)
}

var execCommand = exec.Command

// DefaultCommander provides a default implementation of Commander.
type DefaultCommander struct {
	istioctl IstioctlBinary
}

func NewDefaultCommander(istioctl IstioctlBinary) DefaultCommander {
	return DefaultCommander{istioctl}
}

func (c *DefaultCommander) Install(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error {

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

	cmd := execCommand(c.istioctl.path, "apply", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")
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

func (c *DefaultCommander) Upgrade(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error {

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

	cmd := execCommand(c.istioctl.path, "upgrade", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")
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

func bufferAndLog(r io.Reader, logger *zap.SugaredLogger) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Info(scanner.Text())
	}
}
