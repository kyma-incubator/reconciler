package main

import (
	"os"

	cmd "github.com/kyma-incubator/reconciler/cmd/config"
	"github.com/kyma-incubator/reconciler/internal/cli"
)

func main() {
	command := cmd.NewCmd(&cli.Options{})
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
