package preaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

const (
	handleJsUpdateName = "handleToggleJSFileStorage"
	serverNameEnv      = "SERVER_NAME"
)

type handleJsUpdate struct {
	kubeClientProvider
}

// newHandleEnableJSFileStorage
func newHandleJsUpdate() *handleJsUpdate {
	return &handleJsUpdate{
		kubeClientProvider: defaultKubeClientProvider,
	}
}

func (r *handleJsUpdate) Execute(context *service.ActionContext, logger *zap.SugaredLogger) error {
	// decorate logger
	logger = logger.With(log.KeyStep, handleJsUpdateName)

	kubeClient, err := r.kubeClientProvider(context, logger)
	if err != nil {
		return err
	}

	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}

	statefulSet, err := clientSet.AppsV1().StatefulSets(namespace).Get(context.Context, statefulSetName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.With(log.KeyReason, "Nats StatefulSet not found").Info("Step skipped")
			return nil
		}
		return err
	}

	tracker, err := progress.NewProgressTracker(clientSet, logger, progress.Config{Interval: progressTrackerInterval, Timeout: progressTrackerTimeout})
	if err != nil {
		return err
	}

	serverNameExists, err := envVarExist(statefulSet, serverNameEnv)
	if err != nil {
		return err
	}

	// Updating the NATS Helm charts to the version 0.17.3 requires some changes in the StatefulSet's Spec.
	// This requires a deletion of the StatefulSet.
	// The SERVER_NAME Env variable can be considered as an indicator for the update. The previous Kyma NATS chart versions didn't have it.
	if !serverNameExists {
		if err := deleteNatsStatefulSet(context, clientSet, tracker, logger, statefulSet, false); err != nil {
			return err
		}
		return nil
	}

	logger.With(log.KeyReason, "No actions needed").Info("Step skipped")
	return nil
}

// envVarExist checks whether a StatefulSet contains an env variable or not.
func envVarExist(statefulSet *appsv1.StatefulSet, envName string) (bool, error) {
	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if strings.EqualFold(env.Name, envName) {
				return true, nil
			}
		}
	}
	return false, nil
}
