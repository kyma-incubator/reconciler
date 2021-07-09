package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cfgCmd "github.com/kyma-incubator/reconciler/cmd/config"
	svcCmd "github.com/kyma-incubator/reconciler/cmd/service"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/app"
	"github.com/kyma-incubator/reconciler/pkg/db"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	envVarPrefix = "RECONCILER"
)

var DefaultConfigFile string

func main() {
	o := &cli.Options{}
	cmd := newCmd(
		o,
		filepath.Base(os.Args[0]),
		"Kyma reconciler CLI",
		"Command line tool to administrate the Kyma reconciler system")

	cmd.AddCommand(cfgCmd.NewCmd(o))
	cmd.AddCommand(svcCmd.NewCmd(o))

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd(o *cli.Options, name, shortDesc, longDesc string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: shortDesc,
		Long:  longDesc,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			//validate given user input
			if err := o.Validate(); err != nil {
				return err
			}
			return initObjectRegistry(o)
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			//shutdown object context
			return o.ObjectRegistry.Close()
		},
		SilenceErrors: false,
		SilenceUsage:  true,
	}
	cobra.OnInitialize(initViper(o))
	cmd.PersistentFlags().StringVarP(&DefaultConfigFile, "config", "c", "configs/reconciler.yaml", `Path to the configuration file.`)
	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "Show detailed information about the executed command actions.")
	cmd.PersistentFlags().BoolVar(&o.NonInteractive, "non-interactive", false, "Enables the non-interactive shell mode")
	cmd.PersistentFlags().BoolP("help", "h", false, "Command help")
	return cmd
}

func initObjectRegistry(o *cli.Options) error {
	//init object registry
	dbConnFact, err := initDbConnectionFactory(o)
	if err != nil {
		return err
	}
	o.ObjectRegistry, err = app.NewObjectRegistry(dbConnFact, o.Verbose)
	return err
}

func initViper(o *cli.Options) func() {
	return func() {
		//read configuration from ENV vars
		viper.SetEnvPrefix(envVarPrefix)
		viper.AutomaticEnv()

		//read configuration from config file
		cfgFile, err := getConfigFile()
		if err != nil {
			o.Logger().Warn(err.Error())
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

func getConfigFile() (string, error) {
	configFile := strings.TrimSpace(viper.GetString("config"))
	if configFile == "" {
		configFile = DefaultConfigFile
	}
	if !file.Exists(configFile) {
		return "", fmt.Errorf("No configuration file found: set environment variable $%s_CONFIG or define it as CLI parameter", envVarPrefix)
	}
	return configFile, nil
}

func initDbConnectionFactory(o *cli.Options) (db.ConnectionFactory, error) {
	dbDriver := viper.GetString("db.driver")
	if dbDriver == "" {
		return nil, fmt.Errorf("No database driver defined")
	}

	switch dbDriver {
	case "postgres":
		return &db.PostgresConnectionFactory{
			Host:     viper.GetString("db.postgres.host"),
			Port:     viper.GetInt("db.postgres.port"),
			Database: viper.GetString("db.postgres.database"),
			User:     viper.GetString("db.postgres.user"),
			Password: viper.GetString("db.postgres.password"),
			SslMode:  viper.GetBool("db.postgres.sslMode"),
			Debug:    o.Verbose,
		}, nil
	default:
		return nil, fmt.Errorf("Database driver '%s' not supported yet", dbDriver)
	}
}
