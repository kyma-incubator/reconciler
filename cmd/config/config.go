package cmd

import (
	"fmt"
	"os"

	getCmd "github.com/kyma-incubator/reconciler/cmd/config/get"
	getBucketCmd "github.com/kyma-incubator/reconciler/cmd/config/get/bucket"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configFile string

func NewCmd(o *cli.Options) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Kyma reconciler configuration.",
		Long: `Administrative tool for the Kyma reconciler configuration management.
* Add, delete or update configuration entries
* Manage the configuration cache
`,
		// Affects children as well
		SilenceErrors: false,
		SilenceUsage:  true,
	}

	//init viper configuration loader
	cobra.OnInitialize(func() {
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			o.Logger.Error(fmt.Sprintf("Configuration file '%s' not found", configFile))
			return
		}

		viper.SetConfigFile(configFile)
		viper.AutomaticEnv()
		if err := viper.ReadInConfig(); err != nil {
			o.Logger.Debug(fmt.Sprintf("Using configuration file '%s'", viper.ConfigFileUsed()))
		}
	})

	//set options
	cmd.PersistentFlags().StringVarP(&configFile, "config", "c", "reconciler.yaml", `Path to the configuration file.`)
	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "Show detailed information about the executed command actions.")
	cmd.PersistentFlags().BoolVar(&o.NonInteractive, "non-interactive", false, "Enables the non-interactive shell mode")
	cmd.PersistentFlags().BoolP("help", "h", false, "Command help")

	//register commands
	getCommand := getCmd.NewCmd()
	getCommand.AddCommand(getBucketCmd.NewCmd(o))
	cmd.AddCommand(getCommand)

	return cmd
}
