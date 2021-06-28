package cmd

import (
	createCmd "github.com/kyma-incubator/reconciler/cmd/config/create"
	createKeyCmd "github.com/kyma-incubator/reconciler/cmd/config/create/key"
	createValueCmd "github.com/kyma-incubator/reconciler/cmd/config/create/value"
	getCmd "github.com/kyma-incubator/reconciler/cmd/config/get"
	getBucketCmd "github.com/kyma-incubator/reconciler/cmd/config/get/bucket"
	getKeyCmd "github.com/kyma-incubator/reconciler/cmd/config/get/key"
	getValueCmd "github.com/kyma-incubator/reconciler/cmd/config/get/value"
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
