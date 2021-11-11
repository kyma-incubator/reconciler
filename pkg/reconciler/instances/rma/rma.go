package rma

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const ReconcilerName = "rma"

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	kubeClient, err := kubeclient.NewInClusterClient(log)
	if err != nil {
		log.Fatalf("Could not initialize in-cluster k8s client: %s")
	}
	reconciler.
		//register reconciler pre-reconcile action (executed BEFORE reconciliation happens) and post-delete action (executed AFTER deletion happens)
		WithPreReconcileAction(NewIntegrationAction("runtime-monitoring-integration-reconcile", kubeClient)).
		WithPostDeleteAction(NewIntegrationAction("runtime-monitoring-integration-delete", kubeClient))
}
