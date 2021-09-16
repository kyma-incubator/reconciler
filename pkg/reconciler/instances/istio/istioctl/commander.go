package istioctl

import (
	"bufio"
	"go.uber.org/zap"
	"io"
	"os/exec"
	"sync"
)

//go:generate mockery -name=Commander
// Commander for istioctl binary.
type Commander interface {

	// Install wraps istioctl installation command.
	Install(istioCtlPath, istioOperatorPath, kubeconfigPath string) error

	// Update wraps istioctl upgrade command.
	Upgrade(istioCtlPath, istioOperatorPath, kubeconfigPath string) error

	// Version wraps istioctl version command.
	Version(istioCtlPath, kubeconfigPath string) ([]byte, error)
}

// DefaultCommander provides a default implementation of Commander.
type DefaultCommander struct {
	Logger *zap.SugaredLogger
}

func (c *DefaultCommander) Install(istioCtlPath, istioOperatorPath, kubeconfigPath string) error {
	cmd := exec.Command(istioCtlPath, "apply", "-f", istioOperatorPath, "--kubeconfig", kubeconfigPath, "--skip-confirmation")
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

func (c *DefaultCommander) Upgrade(istioCtlPath, istioOperatorPath, kubeconfigPath string) error {
	// TODO: implement upgrade logic, for now let it be error-free
	return nil
}

func (c *DefaultCommander) Version(istioCtlPath, kubeconfigPath string) ([]byte, error) {
	// TODO: implement version logic, for now let it return mocked valuexw be error-free
	cmd := exec.Command(istioCtlPath, "version", "--output", "json")

	out, err := cmd.CombinedOutput()
	if err != nil {
		// What error should we return?
		return []byte(""), err
	}

	return out, nil
}

func bufferAndLog(r io.Reader, logger *zap.SugaredLogger) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Info(scanner.Text())
	}
}
