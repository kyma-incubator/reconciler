package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/pkg/errors"

	//Register all reconcilers
	_ "github.com/kyma-incubator/reconciler/pkg/reconciler/instances"
	"github.com/kyma-incubator/reconciler/pkg/scheduler"
	"github.com/spf13/cobra"
)

type Options struct {
	*cli.Options
	kubeconfigFile string
	kubeconfig     string
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o,
		"", // kubeconfigFile
		"", // kubeconfig
	}
}
func (o *Options) Kubeconfig() string {
	return o.kubeconfig
}

func (o *Options) Validate() error {
	err := o.Options.Validate()
	if err != nil {
		return err
	}
	if o.kubeconfigFile == "" {
		envKubeconfig, ok := os.LookupEnv("KUBECONFIG")
		if !ok {
			return fmt.Errorf("KUBECONFIG environment variable and kubeconfig flag is missing")
		}
		o.kubeconfigFile = envKubeconfig
	}
	if !file.Exists(o.kubeconfigFile) {
		return fmt.Errorf("Reference kubeconfig file '%s' not found", o.kubeconfigFile)
	}
	content, err := ioutil.ReadFile(o.kubeconfigFile)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to read kubeconfig file '%s'", o.kubeconfigFile))
	}
	o.kubeconfig = string(content)
	return nil
}

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Start local Kyma reconciler",
		Long:  "Start local Kyma reconciler",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return RunLocal(o)
		},
	}
	cmd.Flags().StringVar(&o.kubeconfigFile, "kubeconfig", "", "Path to kubeconfig file")
	return cmd
}

func RunLocal(o *Options) error {

	l := logger.NewOptionalLogger(o.Verbose)
	l.Infof("Local installation started with kubeconfig %s", o.kubeconfigFile)

	operationsRegistry := scheduler.NewDefaultOperationsRegistry()

	workerFactory, _ := scheduler.NewLocalWorkerFactory(
		&cluster.MockInventory{},
		operationsRegistry,
		func(component string, status reconciler.Status) {
			l.Infof("Component %s has status %s", component, status)
		},
		true)

	ls, _ := scheduler.NewLocalScheduler(keb.Cluster{
		Kubeconfig: o.kubeconfig,
		KymaConfig: keb.KymaConfig{
			Version: "main",
			Profile: "evaluation",
			Components: []keb.Components{
				{Component: "cluster-essentials", Namespace: "kyma-system"},
				{Component: "istio", Namespace: "istio-system"},
				{Component: "serverless", Namespace: "kyma-system"},
			}}}, workerFactory, true)
	return ls.Run(context.Background())
}
