package scheduler

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
)

type InvokeParams struct {
	ComponentToReconcile *keb.Component
	ComponentsReady      []string
	ClusterState         cluster.State
	SchedulingID         string
	CorrelationID        string
	ReconcilerURL        string
}

func (p *InvokeParams) CreateLocalReconciliation(callbackFunc func(msg *reconciler.CallbackMessage) error) *reconciler.Reconciliation {
	model := p.createReconciliationModel()
	model.CallbackFunc = callbackFunc
	return model
}

func (p *InvokeParams) CreateRemoteReconciliation(callbackURL string) *reconciler.Reconciliation {
	model := p.createReconciliationModel()
	model.CallbackURL = callbackURL
	return model
}

func (p *InvokeParams) createReconciliationModel() *reconciler.Reconciliation {
	version := p.ClusterState.Configuration.KymaVersion
	if p.ComponentToReconcile.Version != "" {
		version = p.ComponentToReconcile.Version
	}

	return &reconciler.Reconciliation{
		ComponentsReady: p.ComponentsReady,
		Component:       p.ComponentToReconcile.Component,
		Namespace:       p.ComponentToReconcile.Namespace,
		Version:         version,
		Profile:         p.ClusterState.Configuration.KymaProfile,
		Configuration:   mapConfiguration(p.ComponentToReconcile.Configuration),
		Kubeconfig:      p.ClusterState.Cluster.Kubeconfig,
		CorrelationID:   p.CorrelationID,
		Repository: reconciler.Repository{
			URL: p.ComponentToReconcile.URL,
		},
	}
}

type reconcilerInvoker interface {
	Invoke(params *InvokeParams) error
}
