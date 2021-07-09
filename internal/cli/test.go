package cli

import "github.com/kyma-incubator/reconciler/pkg/db"

func NewTestOptions() (*Options, error) {
	dbConnFac, err := db.NewTestConnectionFactory()
	if err != nil {
		return nil, err
	}
	cliOptions := &Options{
		Verbose: true,
	}
	cliOptions.Init(dbConnFac)
	return cliOptions, nil
}
