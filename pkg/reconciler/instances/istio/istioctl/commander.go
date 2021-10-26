package istioctl

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	istioctlBinaryPathEnvKey = "ISTIOCTL_PATH"
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
type DefaultCommander struct{}

func (c *DefaultCommander) Uninstall(kubeconfig string, logger *zap.SugaredLogger) error {
	istioctlPath, err := resolveIstioctlPath()
	if err != nil {
		return err
	}

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

	cmd := execCommand(istioctlPath, "x", "uninstall", "--purge", "--kubeconfig", kubeconfigPath, "--skip-confirmation")

	return c.execute(cmd, logger)
}

func (c *DefaultCommander) Install(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error {
	istioctlPath, err := resolveIstioctlPath()
	if err != nil {
		return err
	}

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

	cmd := execCommand(istioctlPath, "apply", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")

	return c.execute(cmd, logger)
}

func (c *DefaultCommander) Upgrade(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error {
	istioctlPath, err := resolveIstioctlPath()
	if err != nil {
		return err
	}

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

	cmd := execCommand(istioctlPath, "upgrade", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")

	return c.execute(cmd, logger)
}

func (c *DefaultCommander) Version(kubeconfig string, logger *zap.SugaredLogger) ([]byte, error) {
	istioctlPath, err := resolveIstioctlPath()
	if err != nil {
		return []byte{}, err
	}

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

	cmd := execCommand(istioctlPath, "version", "--output", "json", "--kubeconfig", kubeconfigPath)
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

func resolveIstioctlPath() (string, error) {
	// TODO: Check if the binary is present and executable
	path := os.Getenv(istioctlBinaryPathEnvKey)
	if path == "" {
		return "", errors.New("Istioctl binary could not be found under ISTIOCTL_PATH env variable")
	}

	return path, nil
}
