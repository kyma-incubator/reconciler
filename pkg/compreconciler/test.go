package compreconciler

import (
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func readKubeconfig(t *testing.T) string {
	kubecfgFile := os.Getenv("KUBECONFIG")
	if !file.Exists(kubecfgFile) {
		require.FailNow(t, "Please set env-var KUBECONFIG before executing this test case")
	}
	kubecfg, err := ioutil.ReadFile(kubecfgFile)
	require.NoError(t, err)
	return string(kubecfg)
}

func readManifest(t *testing.T) string {
	manifest, err := ioutil.ReadFile(filepath.Join("test", "unittest.yaml"))
	require.NoError(t, err)
	return string(manifest)
}
