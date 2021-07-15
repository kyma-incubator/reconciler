package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

const (
	paramContractVersion = "contractVersion"
	paramCluster         = "cluster"
	paramConfigVersion   = "configVersion"
	paramOffset          = "offset"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the reconciler service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o)
		},
	}
	cmd.Flags().IntVar(&o.Port, "port", 8080, "Webserver port")
	cmd.Flags().StringVar(&o.SSLCrt, "crt", "", "Path to SSL certificate file")
	cmd.Flags().StringVar(&o.SSLKey, "key", "", "Path to SSL key file")
	return cmd
}

func Run(o *Options) error {
	var err error
	//listen on os events
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	//create context
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		oscall := <-c
		if oscall == os.Interrupt {
			cancel()
		}
	}()

	err = startWebserver(o, ctx)
	if err != nil {
		return err
	}

	err = startScheduler(o, ctx)
	if err != nil {
		return err
	}

	return nil
}
