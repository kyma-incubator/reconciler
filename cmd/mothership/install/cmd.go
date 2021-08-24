package cmd

import (
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"path/filepath"
)

func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Kyma mothershop reconciler",
		Long:  "CLI tool for installing the Kyma mothership reconciler",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o)
		},
	}
	return cmd
}

func Run(o *cli.Options) error {
	o.Logger().Infof("Generating database encryption key")
	encKey, err := db.NewEncryptionKey()
	if err != nil {
		return err
	}

	keyFile := viper.GetString("db.encryption.keyFile")
	keyFileAbs := filepath.Join(filepath.Dir(viper.ConfigFileUsed()), keyFile)
	o.Logger().Infof("Creating key file '%s'", keyFileAbs)
	return ioutil.WriteFile(keyFileAbs, []byte(encKey), 0600)
}
