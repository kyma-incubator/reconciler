package istioctl

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/file"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"os"
	"bufio"
	"go.uber.org/zap"
	"io"
	"os/exec"
	"sync"
)

const (
	istioctlBinaryPathEnvKey = "ISTIOCTL_PATH"
)

//go:generate mockery -name=Commander -outpkg=mock -case=underscore
// Commander for istioctl binary.
type Commander interface {

	// Install wraps istioctl installation command.
	Install(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error

	// Upgrade wraps istioctl upgrade command.
	Upgrade(istioOperator, kubeconfig string, logger *zap.SugaredLogger) error

	// Version wraps istioctl version command.
	Version(kubeconfig string, logger *zap.SugaredLogger) ([]byte, error)
}

// DefaultCommander provides a default implementation of Commander.
type DefaultCommander struct {
	Logger *zap.SugaredLogger
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

	cmd := exec.Command(istioctlPath, "apply", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")
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
		bufferAndLog(stdout, c.Logger)
	}()
	go func() {
		defer wg.Done()
		bufferAndLog(stderr, c.Logger)
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
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

	cmd := exec.Command(istioctlPath, "upgrade", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
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

	cmd := exec.Command(istioctlPath, "version", "--output", "json", "--kubeconfig", kubeconfigPath)
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

func resolveIstioctlPath() (string, error) {
	path := os.Getenv(istioctlBinaryPathEnvKey)
	if path == "" {
		return "", errors.New("Istioctl binary could not be found under ISTIOCTL_PATH env variable")
	}

	return path, nil
}
