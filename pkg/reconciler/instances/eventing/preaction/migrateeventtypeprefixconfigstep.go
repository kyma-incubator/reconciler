package preaction

import (
	"strings"
	"time"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/action"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	migrateEventTypePrefixConfigStepName = "MigrateEventTypePrefixConfig"

	namespace = "kyma-system"

	controllerDeploymentName          = "eventing-controller"
	controllerDeploymentContainerName = "controller"
	controllerDeploymentEnvName       = "EVENT_TYPE_PREFIX"

	publisherDeploymentName          = "eventing-publisher-proxy"
	publisherDeploymentContainerName = "eventing-publisher-proxy"
	publisherDeploymentEnvName       = "LEGACY_EVENT_TYPE_PREFIX"

	configMapName = "eventing"
	configMapKey  = "eventTypePrefix"

	managedByLabelKey   = "reconciler.kyma-project.io/managed-by"
	managedByLabelValue = "reconciler"

	progressTrackerInterval = 5 * time.Second
	progressTrackerTimeout  = 2 * time.Minute
)

// compile-time check
var _ action.Step = &migrateEventTypePrefixConfigStep{}

// migrateEventTypePrefixConfigStep represents a PreAction step that upgrades Kyma Eventing from version 1.X to 2.X.
// This is achieved by making sure that the Eventing controller and publisher do not have the old environment
// variables from Kyma 1.X Eventing, which would prevent upgrading to Kyma 2.X Eventing.
type migrateEventTypePrefixConfigStep struct{}

// Execute the migrateEventTypePrefixConfigStep and returns an error upon failure.
func (m *migrateEventTypePrefixConfigStep) Execute(context *service.ActionContext, logger *zap.SugaredLogger) (err error) {
	// decorate logger
	logger = logger.With(log.KeyStep, migrateEventTypePrefixConfigStepName)

	// prepare Kubernetes clientset
	var clientset kubernetes.Interface
	if clientset, err = context.KubeClient.Clientset(); err != nil {
		return err
	}

	// get the Eventing controller deployment
	var controllerDeployment *v1.Deployment
	if controllerDeployment, err = getDeployment(context, clientset, controllerDeploymentName); err != nil {
		return err
	}

	// skip action if the Eventing controller deployment is already managed by the Kyma reconciler
	// this means that Kyma Eventing is already upgraded to version 2.X
	if controllerDeployment != nil && controllerDeployment.Labels[managedByLabelKey] == managedByLabelValue {
		logger.With(log.KeyReason, "Eventing controller deployment is already managed by Kyma reconciler").Info("Step skipped")
		return nil
	}

	// get the Eventing publisher deployment
	var publisherDeployment *v1.Deployment
	if publisherDeployment, err = getDeployment(context, clientset, publisherDeploymentName); err != nil {
		return err
	}

	// skip action if Eventing is not installed
	if publisherDeployment == nil && controllerDeployment == nil {
		logger.With(log.KeyReason, "Eventing is not installed").Info("Step skipped")
		return nil
	}

	// prepare progress tracker for the Eventing controller and publisher deployments
	tracker, err := getDeploymentProgressTracker(clientset, logger, publisherDeployment, controllerDeployment)
	if err != nil {
		return err
	}

	// check the current state of the Eventing publisher deployment
	if publisherDeployment != nil && !containerHasDesiredEnvValue(publisherDeployment, publisherDeploymentContainerName, publisherDeploymentEnvName, configMapName, configMapKey) {
		return deleteDeploymentsAndWait(context, clientset, logger, tracker, publisherDeployment, controllerDeployment)
	}

	// check the current state of the Eventing controller deployment
	if controllerDeployment != nil && !containerHasDesiredEnvValue(controllerDeployment, controllerDeploymentContainerName, controllerDeploymentEnvName, configMapName, configMapKey) {
		return deleteDeploymentsAndWait(context, clientset, logger, tracker, publisherDeployment, controllerDeployment)
	}

	// current state of the Eventing controller and publisher deployments is matching the desired state
	logger.With(log.KeyReason, "desired state and actual state are matching").Info("Step skipped")
	return nil
}

// getDeployment returns a Kubernetes deployment given its name.
func getDeployment(context *service.ActionContext, clientset kubernetes.Interface, name string) (*v1.Deployment, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.Context, name, metav1.GetOptions{})
	if err == nil {
		return deployment, nil
	}
	if errors.IsNotFound(err) {
		return nil, nil
	}
	return nil, err
}

// getDeploymentProgressTracker returns a progress tracker for the given deployments.
func getDeploymentProgressTracker(clientset kubernetes.Interface, log *zap.SugaredLogger, deployments ...*v1.Deployment) (*progress.Tracker, error) {
	tracker, err := progress.NewProgressTracker(clientset, log, progress.Config{Interval: progressTrackerInterval, Timeout: progressTrackerTimeout})
	if err != nil {
		return nil, err
	}

	for _, deployment := range deployments {
		if deployment == nil {
			continue
		}
		tracker.AddResource(progress.Deployment, deployment.Namespace, deployment.Name)
	}

	return tracker, nil
}

// containerHasDesiredEnvValue returns true if the deployment container env.ValueFrom matches the given configmap name and key, otherwise returns false.
func containerHasDesiredEnvValue(deployment *v1.Deployment, containerName, envName, configMapName, configMapKey string) bool {
	if containerHasEnvValueAsNonEmptyString(deployment, containerName, envName) {
		return false
	}

	return containerHasEnvValueFromConfigMap(deployment, containerName, envName, configMapName, configMapKey)
}

// containerHasEnvValueAsNonEmptyString returns true if the deployment container env.Value is a non-empty string, otherwise returns false.
func containerHasEnvValueAsNonEmptyString(deployment *v1.Deployment, containerName, envName string) bool {
	container := findContainerByName(deployment, containerName)
	if container == nil {
		return false
	}

	for _, env := range container.Env {
		if env.Name != envName {
			continue
		}
		if len(strings.TrimSpace(env.Value)) > 0 {
			return true
		}
	}

	return false
}

// containerHasEnvValueFromConfigMap returns true if the deployment container env.ValueFrom matches the given configmap name and key, otherwise returns false.
func containerHasEnvValueFromConfigMap(deployment *v1.Deployment, containerName, envName, configMapName, configMapKey string) bool {
	container := findContainerByName(deployment, containerName)
	if container == nil {
		return false
	}

	for _, env := range container.Env {
		if env.Name != envName {
			continue
		}
		if env.ValueFrom == nil {
			continue
		}
		if env.ValueFrom.ConfigMapKeyRef.Name != configMapName {
			continue
		}
		if env.ValueFrom.ConfigMapKeyRef.Key != configMapKey {
			continue
		}
		return true
	}

	return false
}

// deleteDeploymentsAndWait deletes the given Kubernetes deployments one by one then blocks
// until the deployments are completely deleted or the timeout is reached.
func deleteDeploymentsAndWait(context *service.ActionContext, clientset kubernetes.Interface, logger *zap.SugaredLogger, tracker *progress.Tracker, deployments ...*v1.Deployment) error {
	logger.With(log.KeyReason, "desired state and actual state are not matching").Info("Step executed")

	for _, deployment := range deployments {
		if deployment == nil {
			continue
		}

		err := clientset.AppsV1().Deployments(deployment.Namespace).Delete(context.Context, deployment.Name, metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}

		logger.Infof("Deployment deleted '%s/%s'", deployment.Namespace, deployment.Name)
	}

	// wait until deployments are completely deleted or timeout
	logger.Info("Waiting for Eventing deployments to be deleted")
	if err := tracker.Watch(context.Context, progress.TerminatedState); err != nil {
		return err
	}
	logger.Info("Eventing deployments are deleted")

	return nil
}

// findContainerByName returns a Kubernetes deployment container given its name.
func findContainerByName(deployment *v1.Deployment, containerName string) *corev1.Container {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return &container
		}
	}
	return nil
}
