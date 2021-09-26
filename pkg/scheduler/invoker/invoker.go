package invoker

import (
	"context"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
)

type Invoker interface {
	Invoke(ctx context.Context, params *Params) error
}

type Params struct {
	ComponentToReconcile *keb.Component
	ComponentsReady      []string
	ClusterState         *cluster.State
	SchedulingID         string
	CorrelationID        string
}

func (p *Params) newLocalReconciliationModel(callbackFunc func(msg *reconciler.CallbackMessage) error) *reconciler.Reconciliation {
	model := p.newReconciliationModel()
	model.CallbackFunc = callbackFunc
	return model
}

func (p *Params) newRemoteReconciliationModel(callbackURL string) *reconciler.Reconciliation {
	model := p.newReconciliationModel()
	model.CallbackURL = callbackURL
	return model
}

func (p *Params) newReconciliationModel() *reconciler.Reconciliation {
	version := p.ClusterState.Configuration.KymaVersion
	if p.ComponentToReconcile.Version != "" {
		version = p.ComponentToReconcile.Version
	}

	configuration := p.ComponentToReconcile.ConfigurationAsMap()
	return &reconciler.Reconciliation{
		ComponentsReady: p.ComponentsReady,
		Component:       p.ComponentToReconcile.Component,
		Namespace:       p.ComponentToReconcile.Namespace,
		Version:         version,
		Profile:         p.ClusterState.Configuration.KymaProfile,
		Configuration:   configuration,
		Kubeconfig:      p.ClusterState.Cluster.Kubeconfig,
		CorrelationID:   p.CorrelationID,
		Repository: &reconciler.Repository{
			URL:            p.ComponentToReconcile.URL,
			TokenNamespace: fmt.Sprint(configuration["repo.token.namespace"]),
		},
	}
}
