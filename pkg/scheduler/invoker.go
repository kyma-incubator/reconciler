package scheduler

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
)

type InvokeParams struct {
	ComponentToReconcile *keb.Components
	ComponentsReady      []string
	ClusterState         cluster.State
	SchedulingID         string
	CorrelationID        string
	ReconcilerURL        string
	InstallCRD           bool
}

type ReconcilerInvoker interface {
	Invoke(params *InvokeParams) error
}


version := params.ClusterState.Configuration.KymaVersion
if params.ComponentToReconcile.Version != "" {
version = params.ComponentToReconcile.Version
}

return componentReconciler.StartLocal(context.Background(), &reconciler.Reconciliation{
ComponentsReady: params.ComponentsReady,
Component:       component,
Namespace:       params.ComponentToReconcile.Namespace,
Version:         version,
Profile:         params.ClusterState.Configuration.KymaProfile,
Configuration:   mapConfiguration(params.ComponentToReconcile.Configuration),
Kubeconfig:      params.ClusterState.Cluster.Kubeconfig,
CallbackFunc: func(status reconciler.Status) error {
if lri.statusFunc != nil {
lri.statusFunc(component, status)
}
