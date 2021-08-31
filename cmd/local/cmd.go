package cmd

import (
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"

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
	cmd.Flags().StringSliceVarP(&o.values, "value", "", []string{}, "Set configuration values. Can specify one or more values, also as a comma-separated list (e.g. --value component.a='1' --value component.b='2' or --value component.a='1',component.b='2').")
	cmd.Flags().StringVar(&o.version, "version", "main", "Kyma version")
	cmd.Flags().StringVar(&o.profile, "profile", "evaluation", "Kyma profile")
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

	ls := scheduler.NewLocalScheduler(workerFactory, scheduler.WithLogger(l))
	return ls.Run(cli.NewContext(), keb.Cluster{
		Kubeconfig: o.kubeconfig,
		KymaConfig: keb.KymaConfig{
			Version:    o.version,
			Profile:    o.profile,
			Components: o.Components()}})
}
