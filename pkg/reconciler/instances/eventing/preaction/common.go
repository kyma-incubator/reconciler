package preaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
	"time"
)

const (
	namespace = "kyma-system"

	progressTrackerInterval = 5 * time.Second
	progressTrackerTimeout  = 2 * time.Minute
)

type kubeClientProvider func(context *service.ActionContext, logger *zap.SugaredLogger) (kubernetes.Client, error)

func defaultKubeClientProvider(context *service.ActionContext, logger *zap.SugaredLogger) (kubernetes.Client, error) {
	kubeClient, err := kubernetes.NewKubernetesClient(context.Task.Kubeconfig, logger, nil)
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}
