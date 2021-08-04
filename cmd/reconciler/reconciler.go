package start

import (
	startCmd "github.com/kyma-incubator/reconciler/cmd/reconciler/start"
	startSvcCmd "github.com/kyma-incubator/reconciler/cmd/reconciler/start/service"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/internal/cli/reconciler"
	reconcilerRegistry "github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/spf13/cobra"

	//register component-reconciler packages (they add themself automatically to the reconciler registry):
	_ "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio"
)

/*
 * PLEASE BE AWARE:
 *
 * Any supported component-reconciler requires an anonymous import statement (see above) to be considered by the CLI!
 *
 */

func NewCmd(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconciler",
		Short: "Administrate Kyma component reconcilers",
		Long:  "Administrative CLI tool for the Kyma component reconcilers",
	}

	reconcilerOpts := reconciler.NewOptions(o) //decorate options with reconciler-specific options

	//create start command
	startCommand := startCmd.NewCmd(reconcilerOpts)
	cmd.AddCommand(startCommand)

	//register component-reconcilers in start-command:
	for _, reconcilerName := range reconcilerRegistry.RegisteredReconcilers() {
		startCommand.AddCommand(startSvcCmd.NewCmd(reconcilerOpts, reconcilerName))
	}

	return cmd
}
