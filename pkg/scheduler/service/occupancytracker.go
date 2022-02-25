package service

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/occupancy"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/worker"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"strings"
	"time"
)

const (
	defaultOccupancyCleanUpInterval  = 5 * time.Minute //default: 5 mins
	defaultOccupancyTrackingInterval = 30 * time.Second
	defaultKcpNamespace              = "kcp-system"
	nameLabelSelector                = "app.kubernetes.io/name"
	mothershipName                   = "mothership"
	mothershipNameSuffix             = "reconciler"
	componentQualifier               = "kyma-project.io/component-reconciler"
	componentLabelSelector           = "component"
)

type OccupancyTracker struct {
	occupancyID              string
	workerPool               *worker.Pool
	repo                     occupancy.Repository
	logger                   *zap.SugaredLogger
	componentReconcilerNames []string
}

func NewOccupancyTracker(workerPool *worker.Pool, repo occupancy.Repository, cfg *config.Config, logger *zap.SugaredLogger) *OccupancyTracker {
	return &OccupancyTracker{
		workerPool:               workerPool,
		repo:                     repo,
		logger:                   logger,
		componentReconcilerNames: getComponentReconcilerNames(cfg),
	}
}

func (t *OccupancyTracker) Run(ctx context.Context) error {
	//using hostname (= pod name) as the id to be able
	//to clean up pods that have died w/o being able to delete their occupancy
	var err error
	t.occupancyID, err = os.Hostname()
	if err != nil {
		return fmt.Errorf("occupancy tracker failed to get host name: %s", err)
	}
	//create in-cluster K8s client
	clientset, err := createK8sInClusterClientSet()
	if err != nil {
		return fmt.Errorf("occupancy tracker failed to create in-cluster clientset: %s", err)
	}
	//start occupancy tracking && cleaning
	trackingTicker := time.NewTicker(defaultOccupancyTrackingInterval)
	cleanupTicker := time.NewTicker(defaultOccupancyCleanUpInterval)
	go func() {
		for {
			select {
			case <-trackingTicker.C:
				runningWorkers, err := t.workerPool.RunningWorkers()
				if err != nil {
					t.logger.Errorf("could not create/update occupancy for %s: %s", t.occupancyID, err)
					break
				}
				poolSize := t.workerPool.Size()
				_, err = t.repo.CreateOrUpdateWorkerPoolOccupancy(t.occupancyID, mothershipName, runningWorkers, poolSize)
				if err != nil {
					t.logger.Errorf("could not create/update occupancy for %s: %s", t.occupancyID, err)
				}
			case <-cleanupTicker.C:
				deletionCnt, err := t.cleanUpOrphanOccupancies(ctx, clientset)
				if err == nil {
					t.logger.Infof("cleaned up %d orphan occupancies successfully", deletionCnt)
				} else {
					t.logger.Errorf("failed to cleaned up orphan occupancies: %s", err)
				}
			case <-ctx.Done():
				t.logger.Info("Deleting Worker Pool Occupancy")
				trackingTicker.Stop()
				cleanupTicker.Stop()
				err = t.repo.RemoveWorkerPoolOccupancy(t.occupancyID)
				if err != nil {
					t.logger.Errorf(err.Error())
				}
				return
			}
		}
	}()
	return nil
}

func createK8sInClusterClientSet() (*kubernetes.Clientset, error) {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(inClusterConfig)
}

func (t *OccupancyTracker) getComponentReconcilerPodNames(ctx context.Context, clientset *kubernetes.Clientset) ([]string, error) {
	componentReconcilersQualifier := fmt.Sprintf("%s=%s", componentQualifier, "")
	commaSeparatedComponentNames := strings.Join(t.componentReconcilerNames, ",")
	componentLabelSelectorValue := fmt.Sprintf("%s in (%s)", componentLabelSelector, commaSeparatedComponentNames)
	return t.getScalablePodNames(ctx, clientset, fmt.Sprintf("%s, %s", componentReconcilersQualifier, componentLabelSelectorValue))
}

func (t *OccupancyTracker) getMotherShipPodNames(ctx context.Context, clientset *kubernetes.Clientset) ([]string, error) {
	labelSelectorValue := fmt.Sprintf("%s-%s", mothershipName, mothershipNameSuffix)
	mothershipLabelSelector := fmt.Sprintf("%s=%s", nameLabelSelector, labelSelectorValue)
	return t.getScalablePodNames(ctx, clientset, mothershipLabelSelector)
}

func (t *OccupancyTracker) getScalablePodNames(ctx context.Context, clientset *kubernetes.Clientset, labelSelector string) ([]string, error) {
	var scalablePodNames []string
	pods, err := clientset.CoreV1().Pods(defaultKcpNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		t.logger.Errorf("occupancy tracker failed to list pods in %s namespace using the %s label selector: %s", defaultKcpNamespace, labelSelector, err)
		return nil, err
	}
	for _, pod := range pods.Items {
		podName := pod.Name
		scalablePodNames = append(scalablePodNames, podName)
	}

	return scalablePodNames, nil
}

func (t *OccupancyTracker) cleanUpOrphanOccupancies(ctx context.Context, clientset *kubernetes.Clientset) (int, error) {
	var occupancyTrackingPods []string
	mothershipPodNames, err := t.getMotherShipPodNames(ctx, clientset)
	if err != nil {
		return 0, err
	}
	componentPodNames, err := t.getComponentReconcilerPodNames(ctx, clientset)
	if err != nil {
		return 0, err
	}
	occupancyTrackingPods = append(componentPodNames, mothershipPodNames...)

	componentIDs, err := t.repo.GetComponentIDs()
	if err != nil {
		return 0, err
	}
	if len(componentIDs) == 0 {
		t.logger.Warnf("occupancy tracker received empty list of ids: nothing to clean")
		return 0, nil
	}
	var idsOfOrphanComponents []string
	for _, componentID := range componentIDs {
		found := false
		for _, occupancyTrackingPod := range occupancyTrackingPods {
			if componentID == occupancyTrackingPod {
				found = true
				break
			}
		}
		if !found {
			idsOfOrphanComponents = append(idsOfOrphanComponents, componentID)
		}
	}
	if len(idsOfOrphanComponents) == 0 {
		t.logger.Warnf("occupancy tracker found 0 orphan occupancies: nothing to clean")
		return 0, nil
	}
	return t.repo.RemoveWorkerPoolOccupancies(idsOfOrphanComponents)
}

func getComponentReconcilerNames(cfg *config.Config) []string {
	componentReconcilerNames := make([]string, 0, len(cfg.Scheduler.Reconcilers))
	for reconciler := range cfg.Scheduler.Reconcilers {
		componentReconcilerNames = append(componentReconcilerNames, reconciler)
	}
	return componentReconcilerNames
}
