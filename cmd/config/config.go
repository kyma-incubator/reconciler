package cmd

import (
	"fmt"
	"os"

	createCmd "github.com/kyma-incubator/reconciler/cmd/config/create"
	createKeyCmd "github.com/kyma-incubator/reconciler/cmd/config/create/key"
	createValueCmd "github.com/kyma-incubator/reconciler/cmd/config/create/value"
	getCmd "github.com/kyma-incubator/reconciler/cmd/config/get"
	getBucketCmd "github.com/kyma-incubator/reconciler/cmd/config/get/bucket"
	getKeyCmd "github.com/kyma-incubator/reconciler/cmd/config/get/key"
	getValueCmd "github.com/kyma-incubator/reconciler/cmd/config/get/value"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	envVarPrefix = "RECONCILER" //TBC: stay with this name?
)

var defaultConfigFile string

func NewCmd(o *cli.Options) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Kyma reconciler configuration.",
		Long:  "Administrative CLI tool for the Kyma reconciler configuration management.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			//validate given user input
			if err := o.Validate(); err != nil {
				return err
			}

			//init db connection factory if cmd (or sub-cmd) has a run-method
			dbConnFact, err := initDbConnectionFactory(o)
			if err != nil {
				return err
			}
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
	cmd.PersistentFlags().StringVarP(&defaultConfigFile, "config", "c", "reconciler.yaml", `Path to the configuration file.`)
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
	createCmd.AddCommand(createValueCmd.NewCmd(createValueCmd.NewOptions(o)))

	return cmd
}

func initViper(o *cli.Options) func() {
	return func() {
		//read configuration from ENV vars
		viper.SetEnvPrefix(envVarPrefix)
		viper.AutomaticEnv()

		//read configuration from config file
		cfgFile := getConfigFile()
		if !fileExists(cfgFile) {
			o.Logger().Warn(fmt.Sprintf("Configuration file '%s' not found", cfgFile))
			return
		}

		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err == nil {
			o.Logger().Debug(fmt.Sprintf("Using configuration file '%s'", viper.ConfigFileUsed()))
		} else {
			o.Logger().Error(fmt.Sprintf("Failed to read configuration file '%s': %s", cfgFile, err))
		}
	}
}

func getConfigFile() string {
	configFileEnv := viper.GetString("config")
	if fileExists(configFileEnv) {
		return configFileEnv
	}
	return defaultConfigFile
}

func initDbConnectionFactory(o *cli.Options) (db.ConnectionFactory, error) {
	dbDriver := viper.GetString("configManagement.db.driver")
	if dbDriver == "" {
		return nil, fmt.Errorf("No database driver defined")
	}

	switch dbDriver {
	case "postgres":
		return &db.PostgresConnectionFactory{
			Host:     viper.GetString("configManagement.db.postgres.host"),
			Port:     viper.GetInt("configManagement.db.postgres.port"),
			Database: viper.GetString("configManagement.db.postgres.database"),
			User:     viper.GetString("configManagement.db.postgres.user"),
			Password: viper.GetString("configManagement.db.postgres.password"),
			SslMode:  viper.GetBool("configManagement.db.postgres.sslMode"),
			Debug:    o.Verbose,
		}, nil
	default:
		return nil, fmt.Errorf("Database driver '%s' not supported yet", dbDriver)
	}
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}
