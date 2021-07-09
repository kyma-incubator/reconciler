package cli

import (
	"github.com/kyma-incubator/reconciler/pkg/app"
	"github.com/kyma-incubator/reconciler/pkg/db"
)

func NewTestOptions() (*Options, error) {
	dbConnFac, err := db.NewTestConnectionFactory()
	if err != nil {
		return nil, err
	}
	cliOptions := &Options{
		Verbose: true,
	}
	if cliOptions.ObjectRegistry, err = app.NewObjectRegistry(dbConnFac, true); err != nil {
		return nil, err
	}
	return cliOptions, nil
}
