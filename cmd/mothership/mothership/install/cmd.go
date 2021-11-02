package cmd

import (
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Kyma mothership reconciler",
		Long:  "CLI tool for installing the Kyma mothership reconciler",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o)
		},
	}
	cmd.Flags().BoolVar(&o.Backup, "backup", true, "Create a backup of the current encryption key file")
	return cmd
}

func Run(o *Options) error {
	encKeyFile, err := cli.NewEncryptionKey(o.Backup)
	if err == nil {
		o.Logger().Infof("New encryption key file created: %s", encKeyFile)
	} else {
		o.Logger().Warnf("Failed to create encryption key file '%s'", encKeyFile)
	}
	return err
}
