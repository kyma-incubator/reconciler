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
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/kyma-incubator/reconciler/pkg/scheduler"
	"github.com/spf13/cobra"
)

const (
	workspaceDir = ".workspace"
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
	//use a global workspace factory to ensure all component-reconcilers are using the same workspace-directory
	//(otherwise each component-reconciler would handle the download of Kyma resources individually which will cause
	//collisions when sharing the same directory)
	wsFact, err := workspace.NewFactory(workspaceDir, l)
	if err != nil {
		return err
	}
	err = service.UseGlobalWorkspaceFactory(wsFact)
	if err != nil {
		return err
	}
	workerFactory, _ := scheduler.NewLocalWorkerFactory(
		&cluster.MockInventory{},
		scheduler.NewDefaultOperationsRegistry(),
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
				{Component: "certificates", Namespace: "istio-system"},
				{Component: "logging", Namespace: "kyma-system"},
				{Component: "tracing", Namespace: "kyma-system"},
				{Component: "kiali", Namespace: "kyma-system"},
				{Component: "monitoring", Namespace: "kyma-system"},
				{Component: "eventing", Namespace: "kyma-system"},
				{Component: "ory", Namespace: "kyma-system"},
				{Component: "api-gateway", Namespace: "kyma-system"},
				{Component: "service-catalog", Namespace: "kyma-system"},
				{Component: "service-catalog-addons", Namespace: "kyma-system"},
				{Component: "rafter", Namespace: "kyma-system"},
				{Component: "helm-broker", Namespace: "kyma-system"},
				{Component: "cluster-users", Namespace: "kyma-system"},
				{Component: "serverless", Namespace: "kyma-system"},
				{Component: "application-connector", Namespace: "kyma-integration"},
			}}}, workerFactory, true)
	return ls.Run(context.Background())
}
