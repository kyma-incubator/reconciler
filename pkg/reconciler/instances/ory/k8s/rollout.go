package k8s

import (
	"context"
	"fmt"
	internalKubernetes "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	tracker "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

const (
	interval = 12 * time.Second
	timeout  = 5 * time.Minute
)

//go:generate mockery --name=RolloutHandler --outpkg=mock --case=underscore
//RolloutHandler exposes functionality to rollout k8s objects
type RolloutHandler interface {
	//Rollout a given deployment and wait till it successfully up
	RolloutAndWaitForDeployment(ctx context.Context, deployment, namespace string, client internalKubernetes.Client, logger *zap.SugaredLogger) error
}

type DefaultRolloutHandler struct{}

func NewDefaultRolloutHandler() *DefaultRolloutHandler {
	return &DefaultRolloutHandler{}
}

func (h *DefaultRolloutHandler) RolloutAndWaitForDeployment(ctx context.Context, deployment string, namespace string,
	client internalKubernetes.Client, logger *zap.SugaredLogger) error {
	data := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().String())
	err := client.PatchUsingStrategy(ctx, "Deployment", deployment, namespace, []byte(data), types.StrategicMergePatchType)
	if err != nil {
		return errors.Wrap(err, "Failed to patch deployment")
	}
	err = waitForDeployment(ctx, deployment, namespace, client, logger)
	if err != nil {
		return errors.Wrap(err, "Failed to wait for deployment to be rolled out")
	}
	return nil
}

func waitForDeployment(ctx context.Context, deployment string, namespace string, client internalKubernetes.Client, logger *zap.SugaredLogger) error {
	clientset, err := client.Clientset()
	if err != nil {
		return errors.Wrap(err, "Failed to read clientset")
	}
	pt, _ := tracker.NewProgressTracker(clientset, logger, tracker.Config{Interval: interval, Timeout: timeout})
	watchable, err2 := tracker.NewWatchableResource("Deployment")
	if err2 == nil {
		logger.Debugf("Register watchable %s '%s' in namespace '%s'", "Deployment", deployment, namespace)
		pt.AddResource(watchable, namespace, deployment)
	} else {
		return errors.Wrap(err2, "Failed to register watchable resources")
	}

	err = pt.Watch(ctx, tracker.ReadyState)
	if err != nil {
		return errors.Wrap(err, "Failed to wait for deployment to be rolled out")
	}
	return nil
}
