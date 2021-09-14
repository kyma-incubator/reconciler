package scheduler

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
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

func (f *InvokeParams) CreateLocalReconciliation(callbackFunc func(msg *reconciler.CallbackMessage) error) *reconciler.Reconciliation {
	payload := f.createReconciliation()
	payload.CallbackFunc = callbackFunc
	return payload
}

func (f *InvokeParams) CreateRemoteReconciliation(callbackURL string) *reconciler.Reconciliation {
	payload := f.createReconciliation()
	payload.CallbackURL = callbackURL
	return payload
}

func (f *InvokeParams) createReconciliation() *reconciler.Reconciliation {
	version := f.ClusterState.Configuration.KymaVersion
	if f.ComponentToReconcile.Version != "" {
		version = f.ComponentToReconcile.Version
	}

	return &reconciler.Reconciliation{
		ComponentsReady: f.ComponentsReady,
		Component:       f.ComponentToReconcile.Component,
		Namespace:       f.ComponentToReconcile.Namespace,
		Version:         version,
		Profile:         f.ClusterState.Configuration.KymaProfile,
		Configuration:   mapConfiguration(f.ComponentToReconcile.Configuration),
		Kubeconfig:      f.ClusterState.Cluster.Kubeconfig,
		InstallCRD:      f.InstallCRD,
		CorrelationID:   f.CorrelationID,
		Repository: reconciler.Repository{
			URL: f.ComponentToReconcile.URL,
		},
	}
}

type reconcilerInvoker interface {
	Invoke(params *InvokeParams) error
}
