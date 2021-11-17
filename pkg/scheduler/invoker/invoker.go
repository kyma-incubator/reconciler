package invoker

import (
	"context"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
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

func (p *Params) newLocalTask(callbackFunc func(msg *reconciler.CallbackMessage) error) *reconciler.Task {
	model := p.newTask()
	model.CallbackFunc = callbackFunc
	return model
}

func (p *Params) newRemoteTask(callbackURL string) *reconciler.Task {
	model := p.newTask()
	model.CallbackURL = callbackURL
	return model
}

func (p *Params) newTask() *reconciler.Task {
	version := p.ClusterState.Configuration.KymaVersion
	if p.ComponentToReconcile.Version != "" {
		version = p.ComponentToReconcile.Version
	}

	configuration := p.ComponentToReconcile.ConfigurationAsMap()
	tokenNamespace := configuration["repo.token.namespace"]
	if tokenNamespace == nil {
		tokenNamespace = ""
	}

	taskType := model.OperationTypeReconcile
	if p.ClusterState.Status.Status.IsDeletion() {
		taskType = model.OperationTypeDelete
	}

	return &reconciler.Task{
		ComponentsReady: p.ComponentsReady,
		Component:       p.ComponentToReconcile.Component,
		Namespace:       p.ComponentToReconcile.Namespace,
		Version:         version,
		URL:             p.ComponentToReconcile.URL,
		Profile:         p.ClusterState.Configuration.KymaProfile,
		Configuration:   configuration,
		Kubeconfig:      p.ClusterState.Cluster.Kubeconfig,
		Metadata:        *p.ClusterState.Cluster.Metadata,
		CorrelationID:   p.CorrelationID,
		Repository: &reconciler.Repository{
			URL:            p.ComponentToReconcile.URL,
			TokenNamespace: fmt.Sprint(tokenNamespace),
		},
		Type: taskType,
	}
}
