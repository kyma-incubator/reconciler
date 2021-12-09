package invoker

import (
	"context"
	"fmt"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
)

const (
	defaultBranch = "main"
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
}

func (p *Params) newLocalTask(callbackFunc func(msg *reconciler.CallbackMessage) error) *reconciler.Task {
	newTask := p.newTask()
	newTask.CallbackFunc = callbackFunc
	return newTask
}

func (p *Params) newRemoteTask(callbackURL string) *reconciler.Task {
	newTask := p.newTask()
	newTask.CallbackURL = callbackURL
	return newTask
}

func (p *Params) newTask() *reconciler.Task {
	version := p.ClusterState.Configuration.KymaVersion
	url := p.ComponentToReconcile.URL
	if url != "" && strings.HasSuffix(url, ".git") && p.ComponentToReconcile.Version == "" {
		version = defaultBranch
	} else if p.ComponentToReconcile.Version != "" {
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
		URL:             url,
		Profile:         p.ClusterState.Configuration.KymaProfile,
		Configuration:   configuration,
		Kubeconfig:      p.ClusterState.Cluster.Kubeconfig,
		Metadata:        *p.ClusterState.Cluster.Metadata,
		CorrelationID:   p.CorrelationID,
		Repository: &reconciler.Repository{
			URL:            url,
			TokenNamespace: fmt.Sprint(tokenNamespace),
		},
		Type: taskType,
		ReconcilerConfiguration: reconciler.ReconcilerConfiguration{
			MaxRetries: p.MaxOperationRetries,
		},
	}
}
