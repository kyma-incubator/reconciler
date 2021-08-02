package example

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func init() {
	log, err := logger.NewLogger(true)
	if err != nil {
		panic(err)
	}

	log.Debug("Initializing component reconciler 'example'")
	reconciler, err := service.NewComponentReconciler("example", "/workspaces/example-component-reconciler", true)
	if err != nil {
		log.Fatalf("Could not create component reconciler: %s", err)
	}

	//configure reconciler
	reconciler.
		//list dependencies (these components have to be available before this component reconciler is able to run)
		WithDependencies("componentX", "componentY", "componentZ").
		//register reconciler pre-action (executed BEFORE reconciliation happens)
		WithPreReconcileAction(&CustomAction{
			name: "pre-action",
		}).
		//register reconciler action (custom reconciliation logic)
		WithReconcileAction(&CustomAction{
			name: "install-action",
		}).
		//register reconciler post-action (executed AFTER reconciliation happened)
		WithPostReconcileAction(&CustomAction{
			name: "post-action",
		})
}

type CustomAction struct {
	name string
}

func (a *CustomAction) Run(version string, kubeClient kubernetes.Client) error {
	if kubeClient == nil {
		return fmt.Errorf("kubeClient is expected but was nil")
	}
	fmt.Printf("Action '%s' called (retrieved version is '%s')\n", a.name, version)
	return nil
}
