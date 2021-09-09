package cli

import (
	"github.com/kyma-incubator/reconciler/pkg/app"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/require"
	"testing"
)

func NewTestOptions(t *testing.T) *Options {
	dbConnFac, err := db.NewTestConnectionFactory()
	require.NoError(t, err)

	cliOptions := &Options{
		Verbose: true,
	}
	cliOptions.Registry, err = app.NewApplicationRegistry(dbConnFac, true)
	require.NoError(t, err)

	return cliOptions
}
