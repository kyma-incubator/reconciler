package executor

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/stretchr/testify/assert"
	"testing"
)

var log = logger.NewLogger(false)

func Test_ErrorWhenNoCommandPassed(t *testing.T) {

	cmdExecutor := DefaultCmdExecutor{}
	err := cmdExecutor.RuntWithRetry(log, "")
	assert.Error(t, err, "cmdName must be not empty")
}

func Test_SuccessfulEchoCmd(t *testing.T) {
	cmdExecutor := DefaultCmdExecutor{}
	err := cmdExecutor.RuntWithRetry(log, "echo", "Hello", "Go")
	assert.NoError(t, err)
}

func Test_UnSuccessfulDummyCmd(t *testing.T) {
	cmdExecutor := DefaultCmdExecutor{}
	err := cmdExecutor.RuntWithRetry(log, "may the fourth")
	assert.Error(t, err)
}
