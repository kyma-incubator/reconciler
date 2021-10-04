package cmd

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	schedulerSvc "github.com/kyma-incubator/reconciler/pkg/scheduler/service"
	"path/filepath"

	"github.com/kyma-incubator/reconciler/internal/cli"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	//Register all reconcilers
	_ "github.com/kyma-incubator/reconciler/pkg/reconciler/instances"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
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
	cmd.Flags().StringVar(&o.componentsFile, "components-file", "", `Path to the components file (default "<workspace>/installation/resources/components-2.0.yaml")`)
	cmd.Flags().StringSliceVar(&o.values, "value", []string{}, "Set configuration values. Can specify one or more values, also as a comma-separated list (e.g. --value component.a='1' --value component.b='2' or --value component.a='1',component.b='2').")
	cmd.Flags().StringVar(&o.version, "version", "main", "Kyma version")
	cmd.Flags().StringVar(&o.profile, "profile", "evaluation", "Kyma profile")
	return cmd
}

func RunLocal(o *Options) error {
	l := logger.NewLogger(o.Verbose)

	l.Infof("Local installation started with kubeconfig %s", o.kubeconfigFile)

	//use a global workspace factory to ensure all component-reconcilers are using the same workspace-directory
	//(otherwise each component-reconciler would handle the download of Kyma resources individually which will cause
	//collisions when sharing the same directory)
	wsFact, err := workspace.NewFactory(nil, workspaceDir, l)
	if err != nil {
		return err
	}
	err = service.UseGlobalWorkspaceFactory(wsFact)
	if err != nil {
		return err
	}

	ws, err := wsFact.Get(o.version)
	if err != nil {
		return err
	}
	defaultComponentsYaml := filepath.Join(ws.InstallationResourceDir, "components.yaml")

	printStatus := func(component string, msg *reconciler.CallbackMessage) {
		l.Infof("Component '%s' has status %s (reason: '%v')", component, msg.Status, msg.Error)
	}

	comps, err := o.Components(defaultComponentsYaml)
	if err != nil {
		return err
	}

	runtimeBuilder := schedulerSvc.NewRuntimeBuilder(reconciliation.NewInMemoryReconciliationRepository(), l)

	return runtimeBuilder.RunLocal([]string{}, printStatus).Run(cli.NewContext(), &cluster.State{
		Cluster: &model.ClusterEntity{
			Version:    1,
			Cluster:    "local",
			Kubeconfig: o.kubeconfig,
			Contract:   1,
		},
		Configuration: &model.ClusterConfigurationEntity{
			Version:        1,
			Cluster:        "local",
			ClusterVersion: 1,
			KymaVersion:    o.version,
			KymaProfile:    o.profile,
			Components:     comps,
			Contract:       1,
		},
		Status: &model.ClusterStatusEntity{
			ID:             1,
			Cluster:        "local",
			ClusterVersion: 1,
			ConfigVersion:  1,
			Status:         model.ClusterStatusReconcilePending,
		},
	})
}
