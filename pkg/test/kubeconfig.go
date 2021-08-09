package test

import (
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)

func ReadKubeconfig(t *testing.T) string {
	if !RunExpensiveTests() {
		t.Fatal("ReadKubeconfig can only be used when expensive tests are enabled")
	}
	kubecfgFile := os.Getenv("KUBECONFIG")
	if !file.Exists(kubecfgFile) {
		require.Fail(t, "Please set env-var KUBECONFIG before executing this test case")
	}
	kubecfg, err := ioutil.ReadFile(kubecfgFile)
	require.NoError(t, err)
	return string(kubecfg)
}
