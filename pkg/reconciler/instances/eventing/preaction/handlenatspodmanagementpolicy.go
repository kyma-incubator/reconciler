package preaction

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

const (
	handleNATSPodManagementPolicyName = "handleNATSPodManagementPolicy"
	statefulSetName                   = "eventing-nats"
	eventingComponentName             = "eventing"
	statefulSetType                   = "StatefulSet"
	podType                           = "Pod"
	podLabel                          = "app.kubernetes.io/name=nats"
)

// Config holds the global configuration values.
type Config struct {
	Global Global
}

// Global configuration of JetStream feature.
type Global struct {
	Jetstream JetStream `yaml:"jetstream"`
}

// JetStream specific values like podManagementPolicy.
type JetStream struct {
	PodManagementPolicy string `yaml:"podManagementPolicy"`
}

type handleNATSPodManagementPolicy struct {
	kubeClientProvider
}

// handleNATSPodManagementPolicy handles the Pod management policy for NATS.
func newHandleNATSPodManagementPolicy() *handleNATSPodManagementPolicy {
	return &handleNATSPodManagementPolicy{
		kubeClientProvider: defaultKubeClientProvider,
	}
}

func (r *handleNATSPodManagementPolicy) getNATSChartPodManagementPolicy(context *service.ActionContext) (string, error) {
	comp := chart.NewComponentBuilder(context.Task.Version, eventingComponentName).
		WithConfiguration(context.Task.Configuration).
		WithNamespace(namespace).
		Build()

	chartValues, err := context.ChartProvider.Configuration(comp)
	if err != nil {
		return "", err
	}

	data, err := yaml.Marshal(chartValues)
	if err != nil {
		return "", err
	}

	values := &Config{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return "", err
	}

	return fmt.Sprint(values.Global.Jetstream.PodManagementPolicy), nil
}

func (r *handleNATSPodManagementPolicy) Execute(context *service.ActionContext, logger *zap.SugaredLogger) error {
	// Decorate the logger.
	logger = logger.With(log.KeyStep, handleNATSPodManagementPolicyName)

	policyInChart, err := r.getNATSChartPodManagementPolicy(context)
	if err != nil {
		return err
	}

	// If the Parallel Pod management policy is not set in NATS helm chart then skip action.
	if policyInChart != string(appsv1.ParallelPodManagement) {
		logger.With(log.KeyReason, "No actions needed as NATS chart policy != parallel").Info("Step skipped")
		return nil
	}

	// Initialize K8s client.
	kubeClient, err := r.kubeClientProvider(context, logger)
	if err != nil {
		return err
	}

	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}

	// Fetch the NATS StatefulSet from K8s.
	statefulSet, err := clientSet.AppsV1().StatefulSets(namespace).Get(context.Context, statefulSetName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.With(log.KeyReason, "NATS StatefulSet not found").Info("Step skipped")
			return nil
		}
		return err
	}

	tracker, err := progress.NewProgressTracker(clientSet, logger, progress.Config{Interval: progressTrackerInterval, Timeout: progressTrackerTimeout})
	if err != nil {
		return err
	}

	// Updating the NATS PodManagementPolicy in the StatefulSet's Spec requires deletion of the StatefulSet and its Pods.
	if statefulSet.Spec.PodManagementPolicy != appsv1.ParallelPodManagement {
		logger.With(log.KeyReason, "NATS Statefulset's PodManagementPolicy != Parallel").Info("Deleting NATS StatefulSet")
		return deleteNATSStatefulSet(context, clientSet, tracker, logger)
	}

	logger.With(log.KeyReason, "No actions needed").Info("Step skipped")
	return nil
}

// deleteNATSStatefulSet delete the NATS StatefulSet and optionally its assigned PVC.
func deleteNATSStatefulSet(ctx *service.ActionContext, clientSet k8s.Interface, tracker *progress.Tracker, logger *zap.SugaredLogger) error {
	// Fetch a list of all Pods as we need to make sure they are deleted as well.
	listOpts := metav1.ListOptions{LabelSelector: podLabel}
	pods, err := clientSet.CoreV1().Pods(namespace).List(ctx.Context, listOpts)
	if err != nil {
		return err
	}

	// Watch all Pods.
	for _, pod := range pods.Items {
		logger.Info(fmt.Sprintf("Watching NATS-server Pod %s to wait for termination", pod.GetName()))
		if err := addToProgressTracker(tracker, logger, podType, pod.GetName()); err != nil {
			return err
		}
	}

	// Watch the StatefulSet.
	if err := addToProgressTracker(tracker, logger, statefulSetType, statefulSetName); err != nil {
		return err
	}

	// Delete the StatefulSet.
	logger.Info("Deleting NATS StatefulSet in order to perform the migration")
	if err := clientSet.AppsV1().StatefulSets(namespace).Delete(ctx.Context, statefulSetName, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// Wait until all the Pods and the StatefulSet is deleted.
	if err := tracker.Watch(ctx.Context, progress.TerminatedState); err != nil {
		logger.Warnf("Watching progress of deleted resources failed: %s", err)
		return err
	}
	return nil
}

// addToProgressTracker adds the given resource to the progress tracker.
func addToProgressTracker(tracker *progress.Tracker, logger *zap.SugaredLogger, resourceType string, resourceName string) error {
	//if resource is watchable, add it to progress tracker
	watchable, err := progress.NewWatchableResource(resourceType)
	//add only watchable resources to progress tracker
	if err == nil {
		tracker.AddResource(watchable, namespace, resourceName)
	} else {
		logger.Infof("Cannot add the resource kind: %s, name: %s to watchables", resourceType, resourceName)
	}
	if err != nil {
		return err
	}
	return nil
}
