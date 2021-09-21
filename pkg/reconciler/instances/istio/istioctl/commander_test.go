package istioctl

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/stretchr/testify/require"
)

const versionOutput = "version 1.11.1"

func TestInstallProcess(t *testing.T) {
	if os.Getenv("GO_WANT_INSTALL_PROCESS") != "1" {
		return
	}
	fmt.Fprint(os.Stdout, versionOutput)
	os.Exit(0)
}

func fakeExecInstallCommand(command string, args ...string) *exec.Cmd {
	fmt.Println(command)
	cs := []string{"-test.run=TestInstallProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_INSTALL_PROCESS=1"}
	return cmd
}

func TestUpgradeProcess(t *testing.T) {
	if os.Getenv("GO_WANT_UPGRADE_PROCESS") != "1" {
		return
	}
	fmt.Fprint(os.Stdout, versionOutput)
	os.Exit(0)
}

func fakeExecUpgradeCommand(command string, args ...string) *exec.Cmd {
	fmt.Println(command)
	cs := []string{"-test.run=TestUpgradeProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_UPGRADE_PROCESS=1"}
	return cmd
}

func TestVersionProcess(t *testing.T) {
	if os.Getenv("GO_WANT_VERSION_PROCESS") != "1" {
		return
	}
	fmt.Fprint(os.Stdout, versionOutput)
	os.Exit(0)
}

func fakeExecVersionCommand(command string, args ...string) *exec.Cmd {
	fmt.Println(command)
	cs := []string{"-test.run=TestVersionProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_VERSION_PROCESS=1"}
	return cmd
}

func Test_DefaultCommander_Install(t *testing.T) {
	execCommand = fakeExecInstallCommand
	kubeconfig := "kubeConfig"
	istioOperator := "istioOperator"
	log, err := logger.NewLogger(false)
	require.NoError(t, err)
	commander := DefaultCommander{}

	t.Run("should not run the install command when istioctl binary could not be found in env", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "")
		require.NoError(t, err)

		// when
		errors := commander.Install(istioOperator, kubeconfig, log)

		/// then
		require.Error(t, errors)
		require.Contains(t, errors.Error(), "Istioctl binary could not be found")
	})

	t.Run("should run the install command when the binary is found", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)

		// when
		errors := commander.Install(istioOperator, kubeconfig, log)

		/// then
		require.NoError(t, errors)
	})
}

func Test_DefaultCommander_Upgrade(t *testing.T) {
	execCommand = fakeExecUpgradeCommand
	kubeconfig := "kubeConfig"
	istioOperator := "istioOperator"
	log, err := logger.NewLogger(false)
	require.NoError(t, err)
	commander := DefaultCommander{}

	t.Run("should not run the upgrade command when istioctl binary could not be found in env", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "")
		require.NoError(t, err)

		// when
		errors := commander.Upgrade(istioOperator, kubeconfig, log)

		/// then
		require.Error(t, errors)
		require.Contains(t, errors.Error(), "Istioctl binary could not be found")
	})

	t.Run("should run the upgrade command when the binary is found", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)

		// when
		errors := commander.Upgrade(istioOperator, kubeconfig, log)

		/// then
		require.NoError(t, errors)
	})
}

func Test_DefaultCommander_Version(t *testing.T) {
	execCommand = fakeExecVersionCommand
	kubeconfig := "kubeConfig"
	log, err := logger.NewLogger(false)
	require.NoError(t, err)
	commander := DefaultCommander{}

	t.Run("should not run the version command when istioctl binary could not be found in env", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "")
		require.NoError(t, err)

		// when
		_, binaryErr := commander.Version(kubeconfig, log)

		/// then
		require.Error(t, binaryErr)
		require.Contains(t, binaryErr.Error(), "Istioctl binary could not be found")
	})

	t.Run("should run the version command when the binary is found", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)

		// when
		got, errors := commander.Version(kubeconfig, log)

		/// then
		require.NoError(t, errors)
		require.EqualValues(t, versionOutput, string(got))
	})
}
