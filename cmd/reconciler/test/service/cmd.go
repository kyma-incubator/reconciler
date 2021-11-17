package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	startSvcCmd "github.com/kyma-incubator/reconciler/cmd/reconciler/start/service"
	"github.com/kyma-incubator/reconciler/internal/cli"

	"github.com/spf13/cobra"
)

func NewCmd(o *Options, reconcilerName string) *cobra.Command {
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

	cmd.Flags().StringVar(&o.Version, "version", "main", "Kyma version")
	cmd.Flags().StringVar(&o.Component, "component", reconcilerName, "Name of the component to reconcile")
	cmd.Flags().StringVar(&o.Namespace, "namespace", "kyma-system", "Namespace of the component")
	cmd.Flags().StringVar(&o.Profile, "profile", "", "Kyma profile")

	return cmd
}

func Run(o *Options, reconcilerName string) error {
	if err := showCurl(o); err != nil {
		return err
	}

	//start component reconciler
	o.Logger().Infof("Starting component reconciler '%s'", reconcilerName)
	ctx := cli.NewContext()

	workerPool, err := startSvcCmd.StartComponentReconciler(ctx, o.Options, reconcilerName)
	if err != nil {
		return err
	}
	return startSvcCmd.StartWebserver(ctx, o.Options, workerPool)
}

func showCurl(o *Options) error {
	kubeConfig, err := readKubeconfigAsJSON()
	if err != nil {
		o.Logger().Errorf("Could not retrieve kubeconfig")
		return err
	}

	model := reconciler.Task{
		ComponentsReady: nil,
		Component:       o.Component,
		Namespace:       o.Namespace,
		Version:         o.Version,
		Profile:         o.Profile,
		Configuration:   nil,
		Kubeconfig:      kubeConfig,
		CallbackURL:     "https://httpbin.org/post",
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

func readKubeconfigAsJSON() (string, error) {
	kubeConfigFile, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		return "", fmt.Errorf("set env-var 'KUBECONFIG' before executing the test command")
	}

	kubeConfig, err := ioutil.ReadFile(kubeConfigFile)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("failed to read kubeconfig file '%s'", kubeConfigFile))
	}

	yamlData := make(map[string]interface{})
	fileExt := filepath.Ext(kubeConfigFile)
	if fileExt == ".yaml" || fileExt == ".yml" {
		//convert YAML to JSON to avoid issues with quotes in the CURL command
		if err := yaml.Unmarshal(kubeConfig, &yamlData); err != nil {
			return "", errors.Wrap(err, fmt.Sprintf("failed to unmarshal YAML kubeconfig file '%s'", kubeConfigFile))
		}
		kubeConfig, err = json.Marshal(yamlData)
		if err != nil {
			return "", errors.Wrap(err, "failed to convert YAML kubeconfig into JSON string")
		}
	} else if fileExt != ".json" {
		//just verify that kubeconfig-file indicates to be a JSON format
		return "", fmt.Errorf("file extension '%s' is not supported as kubeconfig file", fileExt)
	}

	return string(kubeConfig), nil
}
