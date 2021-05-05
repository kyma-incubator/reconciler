package cmd

import (
	"fmt"
	"os"

	createCmd "github.com/kyma-incubator/reconciler/cmd/config/create"
	createKeyCmd "github.com/kyma-incubator/reconciler/cmd/config/create/key"
	getCmd "github.com/kyma-incubator/reconciler/cmd/config/get"
	getBucketCmd "github.com/kyma-incubator/reconciler/cmd/config/get/bucket"
	getKeyCmd "github.com/kyma-incubator/reconciler/cmd/config/get/key"
	getValueCmd "github.com/kyma-incubator/reconciler/cmd/config/get/value"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configFile string

func NewCmd(o *cli.Options) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Kyma reconciler configuration.",
		Long:  "Administrative CLI tool for the Kyma reconciler configuration management.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			//init db connection factory if cmd (or sub-cmd) has a run-method
			dbConnFact, err := initDbConnectionFactory()
			o.Init(dbConnFact)
			return err
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			//close db connection factory after cmd (or sub-cmd) was executed
			return o.Close()
		},
		SilenceErrors: false,
		SilenceUsage:  true,
	}

	//init viper configuration loader
	cobra.OnInitialize(initViper(o))

	//set options
	cmd.PersistentFlags().StringVarP(&configFile, "config", "c", "reconiler.yaml", `Path to the configuration file.`)
	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "Show detailed information about the executed command actions.")
	cmd.PersistentFlags().BoolVar(&o.NonInteractive, "non-interactive", false, "Enables the non-interactive shell mode")
	cmd.PersistentFlags().BoolP("help", "h", false, "Command help")

	//register get commands
	getCmd := getCmd.NewCmd(o)
	cmd.AddCommand(getCmd)
	getCmd.AddCommand(getBucketCmd.NewCmd(o))
	getCmd.AddCommand(getKeyCmd.NewCmd(getKeyCmd.NewOptions(o)))
	getCmd.AddCommand(getValueCmd.NewCmd(getValueCmd.NewOptions(o)))

	//register create commands
	createCmd := createCmd.NewCmd(o)
	cmd.AddCommand(createCmd)
	createCmd.AddCommand(createKeyCmd.NewCmd(createKeyCmd.NewOptions(o)))

	return cmd
}

func initViper(o *cli.Options) func() {
	return func() {
		//read configuration from ENV vars
		viper.SetEnvPrefix("RECONF_") //TODO: stay with this name?
		viper.AutomaticEnv()

		//read configuration from config file
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			o.Logger().Error(fmt.Sprintf("Configuration file '%s' not found", configFile))
		} else {
			viper.SetConfigFile(configFile)
			if err := viper.ReadInConfig(); err == nil {
				o.Logger().Debug(fmt.Sprintf("Using configuration file '%s'", viper.ConfigFileUsed()))
			} else {
				o.Logger().Error(fmt.Sprintf("Failed to read configuration file '%s': %s", configFile, err))
			}
		}

	}
}

func initDbConnectionFactory() (db.ConnectionFactory, error) {
	dbDriver := viper.GetString("config.db.driver")
	if dbDriver == "" {
		return nil, fmt.Errorf("No database driver defined")
	}

	switch dbDriver {
	case "postgres":
		return &db.PostgresConnectionFactory{
			Host:     viper.GetString("config.db.postgres.host"),
			Port:     viper.GetInt("config.db.postgres.port"),
			Database: viper.GetString("config.db.postgres.database"),
			User:     viper.GetString("config.db.postgres.user"),
			Password: viper.GetString("config.db.postgres.password"),
			SslMode:  viper.GetBool("config.db.postgres.sslMode"),
		}, nil
	default:
		return nil, fmt.Errorf("Database driver '%s' not supported yet", dbDriver)
	}
}
