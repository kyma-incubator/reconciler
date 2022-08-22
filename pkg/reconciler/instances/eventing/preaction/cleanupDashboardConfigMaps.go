package preaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	configMapNames = []string{"eventing-dashboards-event-types-summary", "eventing-dashboards-delivery-per-subscription"}
)

type handleCleanupDashboardConfigMaps struct {
	kubeClientProvider
}

// newHandleCleanupDashboardConfigMaps
func newHandleCleanupDashboardConfigMaps() *handleCleanupDashboardConfigMaps {
	return &handleCleanupDashboardConfigMaps{
		kubeClientProvider: defaultKubeClientProvider,
	}
}

func (r *handleCleanupDashboardConfigMaps) Execute(context *service.ActionContext, logger *zap.SugaredLogger) error {
	// decorate logger
	logger = logger.With(log.KeyStep, handleEnableJSFileStorageName)

	kubeClient, err := r.kubeClientProvider(context, logger)
	if err != nil {
		return err
	}

	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}

	for _, configMapName := range configMapNames {
		if err := clientSet.CoreV1().ConfigMaps(namespace).Delete(context.Context, configMapName, metav1.DeleteOptions{}); err != nil {
			if k8serrors.IsNotFound(err) {
				logger.Infof("ConfigMap %s not found", configMapName)
			} else {
				logger.Infof("Failed to delete ConfigMap %s due to error: %s", configMapName, err.Error())
			}
		} else {
			logger.Infof("ConfigMap %s deleted", configMapName)
		}
	}

	return nil
}
