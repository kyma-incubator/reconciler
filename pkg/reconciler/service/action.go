package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"go.uber.org/zap"
)

type ActionContext struct {
	KubeClient       kubernetes.Client
	WorkspaceFactory *workspace.Factory
	Context          context.Context
	Logger           *zap.SugaredLogger
	ChartProvider    *chart.Provider
}

type Action interface {
	Run(version, profile string, configuration []reconciler.Configuration, helper *ActionContext) error
}
