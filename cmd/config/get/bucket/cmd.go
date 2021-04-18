package cmd

import (
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/config"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/spf13/cobra"
)

//NewCmd creates a new apply command
func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bucket",
		Short: "Get configuration entry bucket(s).",
		Long:  `Get available buckets and their containing configuration entries.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(o)
		},
	}
	return cmd
}

func Run(o *cli.Options) error {

	pgConnFac := &db.PostgresConnectionFactory{
		Host:     "localhost",
		Database: "kyma",
		User:     "kyma",
		Password: "kyma",
		SslMode:  false,
	}
	cfgRepo, err := config.NewConfigEntryRepository(pgConnFac)
	if err != nil {
		panic(err)
	}

	return cfgRepo.Close()
}
