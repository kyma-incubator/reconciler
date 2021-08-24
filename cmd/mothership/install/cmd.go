package cmd

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/db"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

func NewCmd(o *Options) *cobra.Command {
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
	cmd.Flags().BoolVar(&o.Backup, "backup", true, "Create a backup of the previous encryption key file")
	return cmd
}

func Run(o *Options) error {
	o.Logger().Infof("Generating database encryption key")
	encKey, err := db.NewEncryptionKey()
	if err != nil {
		return err
	}

	keyFile := viper.GetString("db.encryption.keyFile")
	keyFileAbs := filepath.Join(filepath.Dir(viper.ConfigFileUsed()), keyFile)

	if file.Exists(keyFileAbs) && o.Backup {
		timestamp := time.Now().Unix()
		encKeyFileBackup := fmt.Sprintf("%s.%d.bak", keyFileAbs, timestamp)
		o.Logger().Infof("Renaming existing encryption key file to '%s'", encKeyFileBackup)
		if err := os.Rename(keyFileAbs, encKeyFileBackup); err != nil {
			return err
		}
	}

	o.Logger().Infof("Storing new encryption key in file '%s'", keyFileAbs)
	return ioutil.WriteFile(keyFileAbs, []byte(encKey), 0600)
}
