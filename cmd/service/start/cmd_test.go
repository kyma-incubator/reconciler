package cmd

import (
	"testing"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestReconcilerWebservice(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}
	cliOptions, err := cli.NewTestOptions()
	require.NoError(t, err)

	o := &Options{cliOptions, 8080, "", ""}
	require.NoError(t, Run(o))
}
