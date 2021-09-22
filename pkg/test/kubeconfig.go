package test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/stretchr/testify/require"
)

func ReadKubeconfig(t *testing.T) string {
	kubecfgFile := os.Getenv("KUBECONFIG")
	if kubecfgFile == "" {
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		kubecfgFile = path.Join(home, ".kube", "config")
	}
	if !file.Exists(kubecfgFile) {
		require.Fail(t, "Please set your default kubeconfig or set the KUBECONFIG env var before executing this test case")
	}
	kubecfg, err := ioutil.ReadFile(kubecfgFile)
	require.NoError(t, err)
	return string(kubecfg)
}
