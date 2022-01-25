package hydra

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/k8s"
	internalKubernetes "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

//go:generate mockery --name=Syncer --outpkg=mock --case=underscore
//Syncer exposes functionality to trigger hydra specific operations
type Syncer interface {
	// TriggerSynchronization triggers the synchronization of OAuth clients between hydra maester and hydra if needed,
	//that is basically the case when hydra pods started earlier than hydra maester pods, then hydra might be out of sync
	//as it has potentially lost the OAuth clients in his DB
	//See also: https://github.com/ory/hydra-maester/tree/master/docs
	TriggerSynchronization(context context.Context, client internalKubernetes.Client, logger *zap.SugaredLogger, namespace string, forceSync bool) error
}

type DefaultHydraSyncer struct {
	rolloutHandler k8s.RolloutHandler
}

// NewDefaultHydraSyncer returns an instance of DefaultHydraSyncer
func NewDefaultHydraSyncer(rolloutHandler k8s.RolloutHandler) *DefaultHydraSyncer {
	return &DefaultHydraSyncer{rolloutHandler: rolloutHandler}
}

const (
	hydraPodName           = "app.kubernetes.io/name=hydra"
	hydraMaesterPodName    = "app.kubernetes.io/name=hydra-maester"
	hydraMaesterDeployment = "ory-hydra-maester"
)

func (c *DefaultHydraSyncer) TriggerSynchronization(context context.Context, client internalKubernetes.Client, logger *zap.SugaredLogger, namespace string, forceSync bool) error {
	clientset, err := client.Clientset()
	if err != nil {
		return errors.Wrap(err, "Failed to read clientset")
	}
	var restartHydraMaester = false
	if !forceSync {
		restartHydraMaester, err = restartHydraMaesterDeploymentNeeded(context, clientset, logger, namespace)
		if err != nil {
			return errors.Wrap(err, "Failed to determine hydra pod status")
		}
	}
	if restartHydraMaester || forceSync {
		logger.Info("Rolling out hydra-maester deployment")
		err = c.rolloutHandler.RolloutAndWaitForDeployment(context, hydraMaesterDeployment, namespace, client, logger)
		if err != nil {
			return errors.Wrap(err, "Failed to rollout ory hydra-maester deployment")
		}
	} else {
		logger.Debug("hydra and hydra-maester are in sync")
	}
	return nil

}

func restartHydraMaesterDeploymentNeeded(context context.Context, client kubernetes.Interface, logger *zap.SugaredLogger, namespace string) (bool, error) {
	earliestHydraPodStartTime, err := getEarliestPodStartTime(context, hydraPodName, client, logger, namespace)
	if err != nil {
		return false, err
	}
	logger.Debugf("Earliest hydra restart time: %s ", earliestHydraPodStartTime.String())

	earliestHydraMaesterPodStartTime, err := getEarliestPodStartTime(context, hydraMaesterPodName, client, logger, namespace)
	if err != nil {
		return false, err
	}
	logger.Debugf("Earliest hydra-maester restart time: %s ", earliestHydraMaesterPodStartTime.String())

	return earliestHydraPodStartTime.After(earliestHydraMaesterPodStartTime), nil
}

func getEarliestPodStartTime(context context.Context, label string, client kubernetes.Interface, logger *zap.SugaredLogger, namespace string) (time.Time, error) {
	maxStartTime := time.Date(9999, 12, 31, 12, 59, 59, 59, time.UTC)
	podList, err := client.CoreV1().Pods(namespace).List(context, metav1.ListOptions{
		LabelSelector: label})
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to read pods for label %s", label)
	}
	if len(podList.Items) < 1 {
		return time.Time{}, errors.Errorf("Could not find pods for label %s in namespace %s", label, namespace)
	}

	earliestPodStartTime := maxStartTime

	for i := range podList.Items {
		pod := podList.Items[i]
		if pod.Status.Phase == corev1.PodRunning {
			logger.Debugf("Retrieved pod with name: %s, creationTime: %s ", podList.Items[i].Name,
				podList.Items[i].CreationTimestamp.String())
			if podList.Items[i].CreationTimestamp.Time.Before(earliestPodStartTime) {
				earliestPodStartTime = podList.Items[i].CreationTimestamp.Time
			}
		}

	}
	if earliestPodStartTime.Equal(maxStartTime) {
		return time.Time{}, errors.Errorf("Could not find any running pod for label %s in namespace %s", label, namespace)
	}
	return earliestPodStartTime, nil
}
