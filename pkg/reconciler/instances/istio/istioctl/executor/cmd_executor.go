package executor

import (
	"fmt"
	"github.com/avast/retry-go"
	gocmd "github.com/go-cmd/cmd"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"strings"
)

//go:generate mockery --name=CmdExecutor --output=mocks --case=underscore
type CmdExecutor interface {
	RuntWithRetry(logger *zap.SugaredLogger, command string, args ...string) error
}
type DefaultCmdExecutor struct{}

func (d *DefaultCmdExecutor) RuntWithRetry(logger *zap.SugaredLogger, cmdName string, arg ...string) error {
	if len(cmdName) < 1 {
		return errors.New("cmdName must be not empty")
	}
	retryable := func() error {
		executableCmd := gocmd.NewCmd(cmdName, arg...)
		// Run and wait for Cmd to return Status
		status := <-executableCmd.Start()
		stdout := strings.Join(status.Stdout, " - ")
		logger.Debugf("executed command %s, got output: %s", executableCmd.Name, stdout)

		if status.Error != nil {
			stderr := strings.Join(status.Stderr, " ")
			errorMsg := fmt.Sprintf("got error executing command %s stderr: %s", executableCmd.Name, stderr)
			return errors.Wrap(status.Error, errorMsg)
		}
		return nil
	}
	err := retry.Do(retryable, retry.Attempts(3))
	return err
}
