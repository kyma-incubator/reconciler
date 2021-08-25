package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"

	"github.com/kyma-incubator/reconciler/internal/cli"
	reconCli "github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	"github.com/spf13/cobra"
)

func NewCmd(o *reconCli.Options, reconcilerName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   reconcilerName,
		Short: fmt.Sprintf("Test '%s' reconciler service", reconcilerName),
		Long:  fmt.Sprintf("CLI tool to test the Kyma '%s' component reconciler service", reconcilerName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o, reconcilerName)
		},
	}

	return cmd
}

func Run(o *reconCli.Options, reconcilerName string) error {
	if err := showCurl(o); err != nil {
		return err
	}

	//start component reconciler
	o.Logger().Infof("Starting component reconciler '%s'", reconcilerName)
	ctx := cli.NewContext()

	if err := startComponentReconciler(ctx, o, reconcilerName); err != nil {
		return err
	}

	//wait until context gets closed
	<-ctx.Done()
	return ctx.Err()
}

func showCurl(o *reconCli.Options) error {
	kubeConfig, err := readKubeconfig()
	if err != nil {
		o.Logger().Errorf("Could not retrieve kubeconfig")
		return err
	}

	model := reconciler.Reconciliation{
		ComponentsReady: nil,
		Component:       "cluster-users",
		Namespace:       "kyma-test",
		Version:         "1.24.0",
		Profile:         "Evaluation",
		Configuration:   nil,
		Kubeconfig:      kubeConfig,
		CallbackURL:     "https://httpbin.org/post",
		InstallCRD:      false,
		CorrelationID:   "1-2-3-4-5",
	}

	payload, err := json.Marshal(model)
	if err != nil {
		return err
	}
	fmt.Printf(`

Execute this command to trigger the component reconciler:

curl --location \
--request PUT 'http://localhost:%d/v1/run' \
--header 'Content-Type: application/json' \
--data-raw '%s'

*********************************************************

`, o.ServerConfig.Port, payload)

	return nil
}

func readKubeconfig() (string, error) {
	kubeConfigFile, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		return "", fmt.Errorf("set env-var 'KUBECONFIG' before executing the test command")
	}
	kubeConfig, err := ioutil.ReadFile(kubeConfigFile)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("Failed to read kubeconfig file '%s'", kubeConfigFile))
	}
	return string(kubeConfig), nil
}

func startComponentReconciler(ctx context.Context, o *reconCli.Options, reconcilerName string) error {
	recon, err := reconCli.NewComponentReconciler(o, reconcilerName)
	if err != nil {
		return err
	}

	//enable debug mode when running in test command
	if err := recon.Debug(); err != nil {
		return err
	}

	go func(ctx context.Context) {
		if err := recon.StartRemote(ctx); err != nil {
			panic(err)
		}
	}(ctx)

	return nil
}
