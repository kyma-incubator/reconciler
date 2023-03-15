package istioctl

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/executor/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	versionOutput = "version 1.11.1"
	kubeconfig    = "kubeConfig"
)

var testArgs []string

func TestExecProcess(t *testing.T) {
	if os.Getenv("GO_WANT_EXEC_PROCESS") != "1" {
		return
	}
	if os.Getenv("COMMAND") == "version" {
		_, _ = fmt.Fprint(os.Stdout, versionOutput)
	}
	os.Exit(0)
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecProcess", "--", command}
	cs = append(cs, args...)
	testArgs = args
	/* #nosec */
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_EXEC_PROCESS=1"}
	cmd.Env = append(cmd.Env, "COMMAND="+args[0])
	return cmd
}

func Test_DefaultCommander_Install(t *testing.T) {
	mockCommandExecutor := mocks.CmdExecutor{}
	mockCommandExecutor.On("RuntWithRetry", mock.Anything, mock.AnythingOfType("string"),
		mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"),
		mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	var commander = DefaultCommander{
		istioctl:        Executable{path: "/bin/istio/istioctl"},
		commandExecutor: &mockCommandExecutor,
	}
	log := logger.NewLogger(false)

	t.Run("should run the apply command", func(t *testing.T) {
		// when
		errors := commander.Install("istioOperator", kubeconfig, log)

		// then
		require.NoError(t, errors)
		mockCommandExecutor.AssertCalled(t, "RuntWithRetry", log, "/bin/istio/istioctl", "apply", "-f",
			mock.AnythingOfType("string"), "--kubeconfig", mock.AnythingOfType("string"), "--skip-confirmation")
	})
}

func Test_DefaultCommander_Uninstall(t *testing.T) {

	mockCommandExecutor := mocks.CmdExecutor{}
	mockCommandExecutor.On("RuntWithRetry", mock.Anything, mock.AnythingOfType("string"),
		mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"),
		mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	var commander = DefaultCommander{
		istioctl:        Executable{path: "/bin/istio/istioctl"},
		commandExecutor: &mockCommandExecutor,
	}
	log := logger.NewLogger(false)

	t.Run("should run the uninstall command", func(t *testing.T) {
		//when
		err := commander.Uninstall(kubeconfig, log)

		// then

		require.NoError(t, err)
		mockCommandExecutor.AssertCalled(t, "RuntWithRetry", log, "/bin/istio/istioctl", "x", "uninstall", "--purge", "--kubeconfig", mock.AnythingOfType("string"), "--skip-confirmation")
	})
}

func Test_DefaultCommander_Upgrade(t *testing.T) {
	mockCommandExecutor := mocks.CmdExecutor{}
	mockCommandExecutor.On("RuntWithRetry", mock.Anything, mock.AnythingOfType("string"),
		mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"),
		mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	var commander = DefaultCommander{
		istioctl:        Executable{path: "/bin/istio/istioctl"},
		commandExecutor: &mockCommandExecutor,
	}
	log := logger.NewLogger(false)

	t.Run("should run the apply command", func(t *testing.T) {
		// when
		errors := commander.Upgrade("istioOperator", kubeconfig, log)

		// then
		require.NoError(t, errors)
		mockCommandExecutor.AssertCalled(t, "RuntWithRetry", log, "/bin/istio/istioctl", "apply", "-f",
			mock.AnythingOfType("string"), "--kubeconfig", mock.AnythingOfType("string"), "--skip-confirmation")
	})
}

func Test_DefaultCommander_Version(t *testing.T) {
	execCommand = fakeExecCommand
	log := logger.NewLogger(false)
	commander := DefaultCommander{}

	t.Run("should run the version command", func(t *testing.T) {
		// when
		got, errors := commander.Version(kubeconfig, log)

		// then
		require.NoError(t, errors)
		require.Contains(t, string(got), versionOutput)
		require.EqualValues(t, testArgs[0], "version")
		require.EqualValues(t, testArgs[2], "json")
		require.EqualValues(t, testArgs[3], "--kubeconfig")
	})
}
