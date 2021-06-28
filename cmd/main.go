package main

import (
	"os"
	"path/filepath"

	cfgCmd "github.com/kyma-incubator/reconciler/cmd/config"
	svcCmd "github.com/kyma-incubator/reconciler/cmd/service"
	"github.com/kyma-incubator/reconciler/internal/cli"
)

func main() {
	o := &cli.Options{}
	cmd := cli.NewCommand(
		o,
		filepath.Base(os.Args[0]),
		"Kyma reconciler CLI",
		"Command line tool to administrate the Kyma reconciler system")

	//register get commands
	cmd.AddCommand(cfgCmd.NewCmd(o))
	cmd.AddCommand(svcCmd.NewCmd(o))
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
