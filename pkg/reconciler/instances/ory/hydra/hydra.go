package hydra

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"time"
)

//go:generate mockery --name=Hydra --outpkg=mock --case=underscore
//Hydra exposes functionality to trigger hydra specific operations
type Hydra interface {
	// TriggerSynchronization triggers the synchronization of OAuth clients between hydra maester and hydra if needed,
	//that is basically the case when hydra pods started earlier than hydra maester pods, then hydra might be out of sync
	//as it has potentially lost the OAuth clients in his DB
	//See also: https://github.com/ory/hydra-maester/tree/master/docs
	TriggerSynchronization(context context.Context, client kubernetes.Interface, logger *zap.SugaredLogger, namespace string) error
}

type DefaultHydraClient struct {
}

// NewDefaultHydraClient returns an instance of DefaultHydraClient
func NewDefaultHydraClient() *DefaultHydraClient {
	return &DefaultHydraClient{}
}

const (
	hydraPodName           = "app.kubernetes.io/name=hydra"
	hydraMaesterPodName    = "app.kubernetes.io/name=hydra-maester"
	hydraMaesterDeployment = "ory-hydra-maester"
)

func (c *DefaultHydraClient) TriggerSynchronization(context context.Context, client kubernetes.Interface,
	logger *zap.SugaredLogger, namespace string) error {
	restartHydraMaesterDeploymentNeeded, err := hydraStartedAfterHydraMaester(context, client, logger, namespace)
	if err != nil {
		return errors.Wrap(err, "Failed to determine hydra pod status")
	}
	if restartHydraMaesterDeploymentNeeded {
		logger.Info("Rolling out hydra-maester deployment")
		data := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, time.Now().String())
		_, err := client.AppsV1().Deployments(namespace).Patch(context, hydraMaesterDeployment,
			types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			return errors.Wrap(err, "Failed to rollout ory hydra-maester deployment")
		}
	} else {
		logger.Debug("hydra and hydra-maester are in sync")
	}
	return nil

}

func hydraStartedAfterHydraMaester(context context.Context, client kubernetes.Interface, logger *zap.SugaredLogger, namespace string) (bool, error) {
	earliestHydraPodStartTime, err := getEarliestPodStartTime(hydraPodName, context, client, logger, namespace)
	if err != nil {
		return false, err
	}
	logger.Debugf("Earliest hydra restart time: %s ", earliestHydraPodStartTime.String())

	earliestHydraMaesterPodStartTime, err := getEarliestPodStartTime(hydraMaesterPodName, context, client, logger, namespace)
	if err != nil {
		return false, err
	}
	logger.Debugf("Earliest hydra-maester restart time: %s ", earliestHydraMaesterPodStartTime.String())

	return earliestHydraPodStartTime.After(earliestHydraMaesterPodStartTime), nil
}

func getEarliestPodStartTime(label string, context context.Context, client kubernetes.Interface, logger *zap.SugaredLogger, namespace string) (time.Time, error) {

	podList, err := client.CoreV1().Pods(namespace).List(context, metav1.ListOptions{
		LabelSelector: label})
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to read pods for label %s", label)
	}
	if len(podList.Items) < 1 {
		return time.Time{}, errors.Errorf("Could not find pods for label %s in namespace %s", label, namespace)
	}

	earliestPodStartTime := time.Date(9999, 12, 31, 12, 59, 59, 59, time.UTC)
	for i := range podList.Items {
		logger.Debugf("Retrieved pod with name: %s, creationTime: %s ", podList.Items[i].Name,
			podList.Items[i].CreationTimestamp.String())
		if podList.Items[i].CreationTimestamp.Time.Before(earliestPodStartTime) {
			earliestPodStartTime = podList.Items[i].CreationTimestamp.Time
		}
	}
	return earliestPodStartTime, nil
}
