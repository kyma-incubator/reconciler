package cmd

import (
	createCmd "github.com/kyma-incubator/reconciler/cmd/mothership/config/create"
	createKeyCmd "github.com/kyma-incubator/reconciler/cmd/mothership/config/create/key"
	createValueCmd "github.com/kyma-incubator/reconciler/cmd/mothership/config/create/value"
	getCmd "github.com/kyma-incubator/reconciler/cmd/mothership/config/get"
	getBucketCmd "github.com/kyma-incubator/reconciler/cmd/mothership/config/get/bucket"
	getKeyCmd "github.com/kyma-incubator/reconciler/cmd/mothership/config/get/key"
	getValueCmd "github.com/kyma-incubator/reconciler/cmd/mothership/config/get/value"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
)

func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Kyma reconciler configuration",
		Long:  "Administrative CLI tool for the Kyma reconciler configuration management",
	}

	//register get commands
	getCommand := getCmd.NewCmd(o)
	cmd.AddCommand(getCommand)
	getCommand.AddCommand(getBucketCmd.NewCmd(o))
	getCommand.AddCommand(getKeyCmd.NewCmd(getKeyCmd.NewOptions(o)))
	getCommand.AddCommand(getValueCmd.NewCmd(getValueCmd.NewOptions(o)))

	//register create commands
	createCommand := createCmd.NewCmd(o)
	cmd.AddCommand(createCommand)
	createCommand.AddCommand(createKeyCmd.NewCmd(createKeyCmd.NewOptions(o)))
	createCommand.AddCommand(createValueCmd.NewCmd(createValueCmd.NewOptions(o)))

	return cmd
}
