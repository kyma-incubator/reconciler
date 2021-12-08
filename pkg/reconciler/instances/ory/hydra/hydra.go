package hydra

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"time"
)

const (
	hydraPodName           = "app.kubernetes.io/name=hydra"
	hydraMaesterPodName    = "app.kubernetes.io/name=hydra-maester"
	hydraMaesterDeployment = "ory-hydra-maester"
)

// TriggerSynchronization triggers the synchronization of OAuth clients between hydra maester and hydra if needed,
//that is basically the case when hydra pods started earlier than hydra maester pods, then hydra might be out of sync
//as it has potentially lost the OAuth clients in his DB
//See also: https://github.com/ory/hydra-maester/tree/master/docs
func TriggerSynchronization(context *service.ActionContext, client kubernetes.Interface, logger *zap.SugaredLogger, namespace string) error {
	restartHydraMaesterDeploymentNeeded, err := hydraStartedAfterHydraMaester(context, client, logger, namespace)
	if err != nil {
		return errors.Wrap(err, "Failed to determine hydra pod status")
	}
	if restartHydraMaesterDeploymentNeeded {
		logger.Info("Rolling out hydra-maester deployment")
		data := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, time.Now().String())
		_, err := client.AppsV1().Deployments(namespace).Patch(context.Context, hydraMaesterDeployment,
			types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			return errors.Wrap(err, "Failed to rollout ory hydra-maester deployment")
		}
	}
	return nil

}

func hydraStartedAfterHydraMaester(context *service.ActionContext, client kubernetes.Interface, logger *zap.SugaredLogger, namespace string) (bool, error) {
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

func getEarliestPodStartTime(label string, context *service.ActionContext, client kubernetes.Interface,
	logger *zap.SugaredLogger, namespace string) (time.Time, error) {

	podList, err := client.CoreV1().Pods(namespace).List(context.Context, metav1.ListOptions{
		LabelSelector: label})
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to read pods for label %s", label)
	}
	earliestPodStartTime := time.Now()

	for i := range podList.Items {

		logger.Debugf("Retrieved pod with name: %s, creationTime: %s ", podList.Items[i].Name,
			podList.Items[i].CreationTimestamp.String())
		if podList.Items[i].CreationTimestamp.Time.Before(earliestPodStartTime) {
			earliestPodStartTime = podList.Items[i].CreationTimestamp.Time
		}
	}
	return earliestPodStartTime, nil
}
