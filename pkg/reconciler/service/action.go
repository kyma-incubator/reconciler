package service

import (
	"context"

	k8s "k8s.io/client-go/kubernetes"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"go.uber.org/zap"
)

type ActionContext struct {
	KubeClient         kubernetes.Client
	WorkspaceFactory   *workspace.Factory
	Context            context.Context
	Logger             *zap.SugaredLogger
	ChartProvider      *chart.Provider
	Kubeconfig         string
	InClusterClientSet k8s.Interface
	ConfigsMap         map[string]interface{}
	ClientSet          k8s.Interface
}

type Action interface {
	Run(version, profile string, configuration []reconciler.Configuration, helper *ActionContext) error
}
