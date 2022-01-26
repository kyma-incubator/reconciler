package preaction

import (
	"context"
	"strings"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"

	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	removeNatsOperatorStepName = "removeNatsOperator"
	natsOperatorLastVersion    = "1.24.7"
	natsSubChartPath           = "eventing/charts/nats"
	eventingNats               = "eventing-nats"
	oldConfigValue             = "global.image.repository"
	newConfigValue             = "eu.gcr.io/kyma-project"
	crdPlural                  = "customresourcedefinitions"
	serviceKind                = "service"
	podKind                    = "pod"
)

var (
	natsOperatorCRDsToDelete = []string{"natsclusters.nats.io", "natsserviceroles.nats.io"}
)

type kubeClientProvider func(context *service.ActionContext, logger *zap.SugaredLogger) (kubernetes.Client, error)

type removeNatsOperatorStep struct {
	kubeClientProvider
}

func newRemoveNatsOperatorStep() *removeNatsOperatorStep {
	return &removeNatsOperatorStep{
		kubeClientProvider: defaultKubeClientProvider,
	}
}

func defaultKubeClientProvider(context *service.ActionContext, logger *zap.SugaredLogger) (kubernetes.Client, error) {
	kubeClient, err := kubernetes.NewKubernetesClient(context.Task.Kubeconfig, logger, nil)
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}

func (r *removeNatsOperatorStep) Execute(context *service.ActionContext, logger *zap.SugaredLogger) error {
	// TODO skip this step for Kyma 2.X when the following issue is done https://github.com/kyma-incubator/reconciler/issues/334

	// decorate logger
	logger = logger.With(log.KeyStep, removeNatsOperatorStepName)

	kubeClient, err := r.kubeClientProvider(context, logger)
	if err != nil {
		return err
	}

	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}

	tracker, err := progress.NewProgressTracker(clientSet, logger, progress.Config{Interval: progressTrackerInterval, Timeout: progressTrackerTimeout})
	if err != nil {
		return err
	}

	if err := r.removeNatsOperatorResources(context, kubeClient, logger, tracker); err != nil {
		return err
	}

	if err := r.removeNatsOperatorCRDs(context.Context, kubeClient, logger); err != nil {
		return err
	}

	return r.removeOrphanedNatsPods(context.Context, kubeClient, logger, tracker)
}

func (r *removeNatsOperatorStep) removeNatsOperatorResources(context *service.ActionContext, kubeClient kubernetes.Client, logger *zap.SugaredLogger, tracker *progress.Tracker) error {
	// get charts from the version 1.2.x, where the NATS-operator resources still exist
	comp := GetResourcesFromVersion(natsOperatorLastVersion, natsSubChartPath)

	manifest, err := context.ChartProvider.RenderManifest(comp)
	if err != nil {
		return err
	}

	// set the right eventing name, which went lost after rendering
	manifest.Manifest = strings.ReplaceAll(manifest.Manifest, natsSubChartPath, eventingNats)

	logger.Info("Removing nats-operator chart resources")

	var statefulSet *v1.StatefulSet
	statefulSet, err = getStatefulSet(context, kubeClient, eventingNats)
	if err != nil {
		return err
	}

	m := []byte(manifest.Manifest)
	natsResources, _ := kubernetes.ToUnstructured(m, true)
	for _, resource := range natsResources {
		// since the old nats-operator was deployed using the k8s deployment
		// do not delete the nats service if there is a statefulSet deployed
		if statefulSet != nil && resource.GetName() == eventingNats && strings.EqualFold(resource.GetKind(), serviceKind) {
			continue
		}

		//if resource is watchable, add it to progress tracker
		watchable, err := progress.NewWatchableResource(resource.GetKind())
		if err == nil { //add only watchable resources to progress tracker
			tracker.AddResource(watchable, namespace, resource.GetName())
		} else {
			logger.Infof("Cannot add the resource kind: %s, name: %s to watchables", resource.GetKind(), resource.GetName())
		}

		logger.Infof("Deleting: kind: %s, name: %s, namespace: %s", resource.GetKind(), resource.GetName(), namespace)
		_, err = kubeClient.DeleteResource(context.Context, resource.GetKind(), resource.GetName(), namespace)
		if err != nil {
			return err
		}
	}

	//wait until all resources were deleted
	if err := tracker.Watch(context.Context, progress.TerminatedState); err != nil {
		logger.Warnf("Watching progress of deleted resources failed: %s", err)
		return err
	}

	return nil
}

// delete the leftover CRDs, which were outside of charts
func (r *removeNatsOperatorStep) removeNatsOperatorCRDs(context context.Context, kubeClient kubernetes.Client, logger *zap.SugaredLogger) error {
	logger.Info("Removing nats-operator CRDs")
	for _, crdName := range natsOperatorCRDsToDelete {
		_, err := kubeClient.DeleteResource(context, crdPlural, crdName, namespace)
		if err != nil && !errors.IsNotFound(err) {
			logger.Errorf("Failed to delete the nats-operator CRDs, name='%s', namespace='%s': %s", crdName, namespace, err)
			return err
		}
		if err == nil {
			logger.Debugf("Deleted %s CRD from %s namespace", crdName, namespace)
		}
	}
	return nil
}

func GetResourcesFromVersion(version, chartPath string) *chart.Component {
	return chart.NewComponentBuilder(version, chartPath).
		WithConfiguration(map[string]interface{}{
			// replace the missing global value, as we are rendering on the subchart level
			oldConfigValue: newConfigValue,
		}).
		WithNamespace(namespace).
		Build()
}

// getStatefulSet returns a Kubernetes statefulSet given its name
func getStatefulSet(context *service.ActionContext, kubeClient kubernetes.Client, name string) (*v1.StatefulSet, error) {
	statefulSet, err := kubeClient.GetStatefulSet(context.Context, name, namespace)
	if err == nil {
		return statefulSet, nil
	}
	if errors.IsNotFound(err) {
		return nil, nil
	}
	return nil, err
}

// removeOrphanedNatsPods removes orphaned NATS pods if any.
func (r *removeNatsOperatorStep) removeOrphanedNatsPods(context context.Context, kubeClient kubernetes.Client, logger *zap.SugaredLogger, tracker *progress.Tracker) error {
	logger.Info("Removing orphaned NATS pods")

	resources, err := kubeClient.ListResource(context, podKind, metav1.ListOptions{LabelSelector: getNatsPodLabelSelector()})
	if err != nil && errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if resources == nil || len(resources.Items) == 0 {
		return nil
	}

	for _, resource := range resources.Items {
		if watchable, err := progress.NewWatchableResource(resource.GetKind()); err != nil {
			logger.Infof("Cannot add the resource kind: %s, name: %s to watchables", resource.GetKind(), resource.GetName())
		} else {
			tracker.AddResource(watchable, namespace, resource.GetName())
		}

		logger.Infof("Deleting: kind: %s, name: %s, namespace: %s", resource.GetKind(), resource.GetName(), namespace)
		if _, err := kubeClient.DeleteResource(context, resource.GetKind(), resource.GetName(), namespace); err != nil {
			return err
		}
	}

	// wait until all orphaned NATS pods are deleted
	if err := tracker.Watch(context, progress.TerminatedState); err != nil {
		logger.Warnf("Watching progress of deleted resources failed: %s", err)
		return err
	}

	return nil
}

// getNatsPodSelectorLabels returns a comma separated string for the NATS pod selector labels.
func getNatsPodLabelSelector() string {
	labels := getNatsPodLabels()
	return k8slabels.SelectorFromSet(labels).String()
}

// getNatsPodLabels returns a map of NATS pod labels.
func getNatsPodLabels() map[string]string {
	return map[string]string{
		"app":                          "nats",
		"nats_cluster":                 "eventing-nats",
		"app.kubernetes.io/managed-by": "Helm",
	}
}
