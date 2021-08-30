package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

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
	components     []string
}

func defaultComponentList() []keb.Components {
	defaultComponents := []string{"cluster-essentials", "istio-configuration@istio-system",
		"certificates@istio-system", "logging", "tracing", "kiali", "monitoring", "eventing", "ory", "api-gateway",
		"service-catalog", "service-catalog-addons", "rafter", "helm-broker", "cluster-users", "serverless",
		"application-connector@kyma-integration"}
	return componentsFromStrings(defaultComponents)

}

func NewOptions(o *cli.Options) *Options {
	return &Options{o,
		"",         // kubeconfigFile
		"",         // kubeconfig
		[]string{}, // components
	}
}
func (o *Options) Kubeconfig() string {
	return o.kubeconfig
}

func componentsFromStrings(list []string) []keb.Components {
	var components []keb.Components
	for _, item := range list {
		s := strings.Split(item, "@")
		name := s[0]
		namespace := "kyma-system"
		if len(s) >= 2 {
			namespace = s[1]
		}
		components = append(components, keb.Components{Component: name, Namespace: namespace})
	}
	return components
}

func (o *Options) Components() []keb.Components {
	components := componentsFromStrings(o.components)
	if len(components) == 0 {
		components = defaultComponentList()
	}
	return components
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
	cmd.Flags().StringSliceVar(&o.components, "components", []string{}, "Comma separated list of components with optional namespace, e.g. serverless,certificates@istio-system,monitoring")
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
			Version:    "main",
			Profile:    "evaluation",
			Components: o.Components()}}, workerFactory, true)
	return ls.Run(context.Background())
}
