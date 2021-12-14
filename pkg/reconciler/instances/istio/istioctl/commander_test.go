package istioctl

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
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
	execCommand = fakeExecCommand
	istioOperator := "istioOperator"
	log := logger.NewLogger(false)
	commander := DefaultCommander{}
	t.Run("should run the install command", func(t *testing.T) {
		// when
		errors := commander.Install(istioOperator, kubeconfig, log)

		// then
		require.NoError(t, errors)
		require.EqualValues(t, testArgs[0], "apply")
		require.EqualValues(t, testArgs[1], "-f")
		require.EqualValues(t, testArgs[3], "--kubeconfig")
		require.EqualValues(t, testArgs[5], "--skip-confirmation")
	})
}

func Test_DefaultCommander_Uninstall(t *testing.T) {
	execCommand = fakeExecCommand
	var commander Commander = &DefaultCommander{}
	kubeconfig := "kubeconfig"
	log := logger.NewLogger(false)

	t.Run("should run the install command", func(t *testing.T) {
		//when
		err := commander.Uninstall(kubeconfig, log)

		// then
		require.NoError(t, err)
		require.EqualValues(t, testArgs[0], "x")
		require.EqualValues(t, testArgs[1], "uninstall")
		require.EqualValues(t, testArgs[2], "--purge")
		require.EqualValues(t, testArgs[5], "--skip-confirmation")
	})
}

func Test_DefaultCommander_Upgrade(t *testing.T) {
	execCommand = fakeExecCommand
	istioOperator := "istioOperator"
	log := logger.NewLogger(false)
	commander := DefaultCommander{}

	t.Run("should run the upgrade command", func(t *testing.T) {
		// when
		errors := commander.Upgrade(istioOperator, kubeconfig, log)

		// then
		require.NoError(t, errors)
		require.EqualValues(t, testArgs[0], "upgrade")
		require.EqualValues(t, testArgs[1], "-f")
		require.EqualValues(t, testArgs[3], "--kubeconfig")
		require.EqualValues(t, testArgs[5], "--skip-confirmation")
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
		require.EqualValues(t, versionOutput, string(got))
		require.EqualValues(t, testArgs[0], "version")
		require.EqualValues(t, testArgs[2], "json")
		require.EqualValues(t, testArgs[3], "--kubeconfig")
	})
}
