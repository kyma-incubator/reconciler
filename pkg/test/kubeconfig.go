package test

import (
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"
)

func ReadKubeconfig(t *testing.T) string {
	//kubecfgFile := os.Getenv("KUBECONFIG")
	//if !file.Exists(kubecfgFile) {
	//	require.Fail(t, "Please set env-var KUBECONFIG before executing this test case")
	//}
	kubecfg, err := ioutil.ReadFile("/Users/rjankowski/Downloads/kc-rj.yaml")
	require.NoError(t, err)
	return string(kubecfg)
}
