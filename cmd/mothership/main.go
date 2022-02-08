package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cfgCmd "github.com/kyma-incubator/reconciler/cmd/mothership/config"
	localCmd "github.com/kyma-incubator/reconciler/cmd/mothership/local"
	msCmd "github.com/kyma-incubator/reconciler/cmd/mothership/mothership"
	"github.com/kyma-incubator/reconciler/internal/cli"
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
	cmd.AddCommand(msCmd.NewCmd(o))
	cmd.AddCommand(localCmd.NewCmd(localCmd.NewOptions(o)))

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
			return o.InitApplicationRegistry(false)
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if o.Registry != nil {
				//shutdown object context
				return o.Registry.Close()
			}
			return nil
		},
		SilenceErrors: false,
		SilenceUsage:  true,
	}
	cobra.OnInitialize(initViper(o))
	cmd.PersistentFlags().StringVarP(&DefaultConfigFile, "config", "c", "configs/reconciler.yaml", `Path to the configuration file`)
	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "Show detailed information about the executed command actions")
	cmd.PersistentFlags().BoolVar(&o.NonInteractive, "non-interactive", false, "Enables the non-interactive shell mode")
	cmd.PersistentFlags().BoolVarP(&o.InitRegistry, "init-registry", "r", false, "Auto-initialize application registry ")
	cmd.PersistentFlags().BoolVarP(&o.OccupancyTracking, "occupancy-tracking", "ot", false, "Activate worker pool occupancy tracking")
	cmd.PersistentFlags().BoolP("help", "h", false, "Command help")
	return cmd
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
			o.Logger().Debugf("Using configuration file '%s'", viper.ConfigFileUsed())
		} else {
			o.Logger().Errorf("Failed to read configuration file '%s': %s", cfgFile, err)
		}
	}
}

func getConfigFile() (string, error) {
	configFile := strings.TrimSpace(viper.GetString("config"))
	if configFile == "" {
		configFile = DefaultConfigFile
	}
	if !file.Exists(configFile) {
		return "", fmt.Errorf("no configuration file found: set environment variable $%s_CONFIG or define it as CLI parameter", envVarPrefix)
	}
	return configFile, nil
}
