package main

import (
	"os"

	cmd "github.com/kyma-incubator/reconciler/cmd/config"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	command := cmd.NewCmd(&cli.Options{
		Logger: logger,
	})

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
