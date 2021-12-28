package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	scheduler "github.com/kyma-incubator/reconciler/pkg/scheduler/service"
	schedulerSvc "github.com/kyma-incubator/reconciler/pkg/scheduler/service"
	"github.com/spf13/cobra"

	//Register all reconcilers
	_ "github.com/kyma-incubator/reconciler/pkg/reconciler/instances"
)

const (
	workspaceDir         = ".workspace"
	clusterStateTemplate = `{
		"Cluster": {
			"Version":1,
			"RuntimeID":"local",
			"Metadata":{},
			"Contract":1
		},
		"Configuration": {
			"Version":1,
			"RuntimeID":"local",
			"KymaVersion": "main",
			"KymaProfile": "evaluation",
			"ClusterVersion":1,			
			"Contract":1
		},
		"Status": {
			"ID":1,
			"RuntimeID":"local",
			"ClusterVersion":1,
			"ConfigVersion":1
		}
	}`
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
	cmd.Flags().StringVar(&o.clusterState, "cluster", clusterStateTemplate, `Set the Cluster State JSON. Use other flags to override fields in the JSON.`)
	cmd.Flags().StringVar(&o.kubeconfigFile, "kubeconfig", "", "Path to kubeconfig file")
	cmd.Flags().StringSliceVar(&o.components, "components", []string{}, "Comma separated list of components with optional namespace, e.g. serverless,certificates@istio-system,monitoring")
	cmd.Flags().StringVar(&o.componentsFile, "components-file", "", `Path to the components file (default "<workspace>/installation/resources/components.yaml")`)
	cmd.Flags().StringSliceVar(&o.values, "value", []string{}, "Set configuration values. Can specify one or more values, also as a comma-separated list (e.g. --value component.a='1' --value component.b='2' or --value component.a='1',component.b='2').")
	cmd.Flags().StringVar(&o.version, "version", "", "Kyma version")
	cmd.Flags().StringVar(&o.profile, "profile", "", "Kyma profile")
	cmd.Flags().BoolVarP(&o.delete, "delete", "d", false, "Provide this flag to do a deletion instead of reconciliation")
	return cmd
}

func RunLocal(o *Options) error {
	l := logger.NewLogger(o.Verbose)

	l.Infof("Local installation started with kubeconfig %s", o.kubeconfigFile)

	cluster, err := prepareClusterState(o)
	if err != nil {
		return err
	}

	//use a global workspace factory to ensure all component-reconcilers are using the same workspace-directory
	//(otherwise each component-reconciler would handle the download of Kyma resources individually which will cause
	//collisions when sharing the same directory)
	wsFact, err := chart.NewFactory(nil, workspaceDir, l)
	if err != nil {
		return err
	}
	err = service.UseGlobalWorkspaceFactory(wsFact)
	if err != nil {
		return err
	}

	ws, err := wsFact.Get(cluster.Configuration.KymaVersion)
	if err != nil {
		return err
	}
	defaultComponentsYaml := filepath.Join(ws.InstallationResourceDir, "components.yaml")

	printStatus := func(component string, msg *reconciler.CallbackMessage) {
		errMsg := ""
		if msg.Error != "" {
			errMsg = fmt.Sprintf(" (reason: %s)", msg.Error)
		}
		l.Infof("Component '%s' has status '%s'%s", component, msg.Status, errMsg)
	}

	preComps, comps, err := o.Components(defaultComponentsYaml, *cluster)
	if err != nil {
		return err
	}
	cluster.Configuration.Components = comps

	runtimeBuilder := schedulerSvc.NewRuntimeBuilder(reconciliation.NewInMemoryReconciliationRepository(), occupancy.NewInMemoryOccupancyRepository(), l)

	status := model.ClusterStatusReconcilePending
	if o.delete {
		status = model.ClusterStatusDeletePending
	}
	reconResult, err := runtimeBuilder.RunLocal(printStatus).
		WithSchedulerConfig(
			&scheduler.SchedulerConfig{
				PreComponents:            preComps,
				InventoryWatchInterval:   0, // not relevant for local (will change with unification of remote/local cases)
				ClusterReconcileInterval: 0, // not relevant for local (will change with unification of remote/local cases)
				ClusterQueueSize:         10,
				DeleteStrategy:           scheduler.DeleteStrategySystem, // local runners always use default (will change with unification of remote/local cases)
			}).
		Run(cli.NewContext(), cluster)
	if err != nil {
		return err //general issue occurred
	}

	if reconResult.GetResult() == model.ClusterStatusReconcileError { //verify reconciliation result
		var failedOpsCnt int
		var failedOps bytes.Buffer
		for _, op := range reconResult.GetOperations() {
			if op.State != model.OperationStateDone {
				failedOps.WriteString(fmt.Sprintf("\n\t- component '%s' failed with status '%s': %s\n",
					op.Component, op.State, op.Reason))
				failedOpsCnt++
			}
		}
		return fmt.Errorf("reconciliation of %d component(s) failed: %s", failedOpsCnt, failedOps.String())
	}

	return nil
}

func prepareClusterState(o *Options) (*cluster.State, error) {
	clusterState := &cluster.State{}
	err := json.Unmarshal([]byte(o.clusterState), clusterState)
	if err != nil {
		return nil, err
	}

	status := model.ClusterStatusReconcilePending
	if o.delete {
		status = model.ClusterStatusDeletePending
	}

	clusterState.Status.Status = status
	clusterState.Cluster.Kubeconfig = o.kubeconfig

	if o.profile != "" {
		clusterState.Configuration.KymaProfile = o.profile
	}
	if o.version != "" {
		clusterState.Configuration.KymaVersion = o.version
	}

	return clusterState, nil
}
