package invoker

import (
	"context"
	"strings"

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
	MaxOperationRetries  int
	Type                 model.OperationType
	Debug                bool
}

func (p *Params) newLocalTask(callbackFunc func(msg *reconciler.CallbackMessage) error) *reconciler.Task {
	task := p.newTask()
	task.CallbackFunc = callbackFunc
	return task
}

func (p *Params) newRemoteTask(callbackURL string) *reconciler.Task {
	task := p.newTask()
	task.CallbackURL = callbackURL
	return task
}

func (p *Params) newTask() *reconciler.Task {
	version := p.ClusterState.Configuration.KymaVersion
	// version := p.ComponentToReconcile.Version
	url := p.ComponentToReconcile.URL
	if url != "" && strings.HasSuffix(url, ".git") {
		version = p.ComponentToReconcile.Version // ok even if it was empty. We handle it later
	} else if p.ComponentToReconcile.Version != "" {
		version = p.ComponentToReconcile.Version
	}

	return &reconciler.Task{
		ComponentsReady: p.ComponentsReady,
		Component:       p.ComponentToReconcile.Component,
		Namespace:       p.ComponentToReconcile.Namespace,
		Version:         version,
		URL:             url,
		Profile:         p.ClusterState.Configuration.KymaProfile,
		Configuration:   p.ComponentToReconcile.ConfigurationAsMap(),
		Kubeconfig:      p.ClusterState.Cluster.Kubeconfig,
		Metadata:        *p.ClusterState.Cluster.Metadata,
		CorrelationID:   p.CorrelationID,
		Repository: &reconciler.Repository{
			URL: url,
		},
		Type: p.Type,
		ComponentConfiguration: reconciler.ComponentConfiguration{
			MaxRetries: p.MaxOperationRetries,
			Debug:      p.Debug,
		},
	}
}
