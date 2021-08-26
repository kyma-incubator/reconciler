package cmd

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	_ "github.com/kyma-incubator/reconciler/pkg/reconciler/instances"
	"github.com/kyma-incubator/reconciler/pkg/scheduler"
	"github.com/spf13/cobra"
)

func NewCmd(o *cli.Options) *cobra.Command {
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

	return cmd
}

func RunLocal(o *cli.Options) error {

	kubecfgFile := os.Getenv("KUBECONFIG")
	kubeconfig, _ := ioutil.ReadFile(kubecfgFile)

	l, _ := logger.NewLogger(false)

	operationsRegistry := scheduler.NewDefaultOperationsRegistry()

	workerFactory, _ := scheduler.NewLocalWorkerFactory(
		&cluster.MockInventory{},
		operationsRegistry,
		func(component string, status reconciler.Status) {
			l.Infof("Component %s has status %s", component, status)
		},
		true)

	ls, _ := scheduler.NewLocalScheduler(keb.Cluster{
		Kubeconfig: string(kubeconfig),
		KymaConfig: keb.KymaConfig{
			Version: "main",
			Profile: "evaluation",
			Components: []keb.Components{
				{Component: "cluster-essentials", Namespace: "kyma-system"},
				{Component: "istio", Namespace: "istio-system"},
				{Component: "serverless", Namespace: "kyma-system"},
			}}}, workerFactory, true)
	ls.Run(context.Background())
	return nil
}
