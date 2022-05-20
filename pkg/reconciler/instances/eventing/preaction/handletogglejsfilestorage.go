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
	"strings"
)

const (
	handleEnableJSFileStorageName = "handleToggleJSFileStorage"
	natsComponentName             = "eventing-nats"
	statefulSetName               = "eventing-nats"
	eventingComponentName         = "eventing"
	fileStorageType               = "file"
	volumeClaimName               = natsComponentName + "-js-pvc"
	statefulSetType               = "StatefulSet"
)

type handleToggleJSFileStorage struct {
	kubeClientProvider
}

// newHandleEnableJSFileStorage
func newHandleToggleJSFileStorage() *handleToggleJSFileStorage {
	return &handleToggleJSFileStorage{
		kubeClientProvider: defaultKubeClientProvider,
	}
}

// Config holds the global configuration values.
type Config struct {
	Global Global
}

// Global configuration of Jetstream feature.
type Global struct {
	Jetstream Jetstream `yaml:"jetstream"`
}

// Jetstream specific values like enabled flag and storage type.
type Jetstream struct {
	Enabled string `yaml:"enabled"`
	Storage string `yaml:"storage"`
}

func (r *handleToggleJSFileStorage) Execute(context *service.ActionContext, logger *zap.SugaredLogger) error {
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

	comp := chart.NewComponentBuilder(context.Task.Version, eventingComponentName).
		WithConfiguration(context.Task.Configuration).
		WithNamespace(namespace).
		Build()

	chartValues, err := context.ChartProvider.Configuration(comp)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(chartValues)
	if err != nil {
		return err
	}

	values := &Config{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return err
	}

	// fetch Jetstream configuration
	jetstreamEnabled := fmt.Sprint(values.Global.Jetstream.Enabled) == "true"
	storageType := fmt.Sprint(values.Global.Jetstream.Storage)

	// check if the Jetstream PVC is in place
	volumeClaimTemplateFound := false
	for _, volumeClaimTemplate := range statefulSet.Spec.VolumeClaimTemplates {
		if strings.EqualFold(volumeClaimTemplate.Name, volumeClaimName) {
			volumeClaimTemplateFound = true
		}
	}

	//Scenario 1: migrate from NATS to NATS Jetstream with file storage
	//as the statefulSet's spec is immutable, we cannot patch in order to add a PVC, hence statefulSet must be recreated
	if jetstreamEnabled && strings.EqualFold(fileStorageType, fmt.Sprint(storageType)) {
		if !volumeClaimTemplateFound {
			if err := deleteNatsStatefulSet(context, clientSet, tracker, logger, statefulSet, false); err != nil {
				return err
			}
		} else {
			logger.With(log.KeyReason, "Nats StatefulSet already contains a PVC").Info("Step skipped")
		}
		return nil
	}

	// Scenario 2: migrate from NATS Jetstream with file storage to NATS
	// as the statefulSet's spec is immutable, we cannot patch it. Hence, in order to remove a PVC, the StatefulSet must be recreated
	if !jetstreamEnabled && volumeClaimTemplateFound {
		if err := deleteNatsStatefulSet(context, clientSet, tracker, logger, statefulSet, true); err != nil {
			return err
		}
		return nil
	}

	logger.With(log.KeyReason, "No actions needed").Info("Step skipped")
	return nil
}

// deleteNatsStatefulSet delete the Nats StatefulSet and optionally its assigned PVC.
func deleteNatsStatefulSet(context *service.ActionContext, clientSet k8s.Interface, tracker *progress.Tracker, logger *zap.SugaredLogger, statefulSet *appsv1.StatefulSet, deletePVC bool) error {
	// In case of switching from Nats Jestream to NATS the PVCs assigned to the Nats Stateful set should be deleted
	if deletePVC {
		err := deleteNatsJsPVC(context, statefulSet, clientSet, logger)
		if err != nil {
			return err
		}
	}

	if err := addToProgressTracker(tracker, logger, statefulSetType, statefulSetName); err != nil {
		return err
	}
	logger.Info("Deleting nats StatefulSet in order to perform the PVC migration")
	if err := clientSet.AppsV1().StatefulSets(namespace).Delete(context.Context, statefulSetName, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// wait until StatefulSet is deleted
	if err := tracker.Watch(context.Context, progress.TerminatedState); err != nil {
		logger.Warnf("Watching progress of deleted resources failed: %s", err)
		return err
	}
	return nil
}

// deleteNatsJsPVC removes the PVCs assigned to nats pods.
func deleteNatsJsPVC(context *service.ActionContext, statefulSet *appsv1.StatefulSet, clientSet k8s.Interface, logger *zap.SugaredLogger) error {
	replicas := statefulSet.Spec.Replicas
	for i := 0; i < int(*replicas); i++ {
		// build the pvc name from the pod's name
		pvcName := volumeClaimName + "-" + statefulSetName + "-" + fmt.Sprint(i)
		pvc, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(context.Context, pvcName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		if pvc != nil {
			logger.Infof("Deleting nats Jestream PVC %s", pvcName)
			if err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Context, pvcName, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
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
