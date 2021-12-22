package k8s

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kctlutil "k8s.io/kubectl/pkg/util/deployment"
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
	RolloutAndWaitForDeployment(ctx context.Context, deployment, namespace string, client kubernetes.Interface, logger *zap.SugaredLogger) error
}

type DefaultRolloutHandler struct{}

func NewDefaultRolloutHandler() *DefaultRolloutHandler {
	return &DefaultRolloutHandler{}
}

func (h *DefaultRolloutHandler) RolloutAndWaitForDeployment(ctx context.Context, deployment string, namespace string, client kubernetes.Interface, logger *zap.SugaredLogger) error {
	data := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`,
		time.Now().String())
	_, err := client.AppsV1().Deployments(namespace).Patch(ctx, deployment, types.StrategicMergePatchType, []byte(data),
		metav1.PatchOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to patch deployment")
	}
	err = waitForDeployment(deployment, namespace, client, logger)
	if err != nil {
		return errors.Wrap(err, "Failed to wait for deployment to be rolled out")
	}
	return nil
}

func waitForDeployment(deployment string, namespace string, client kubernetes.Interface, logger *zap.SugaredLogger) error {
	logger.Debugf("Waiting for deployment to be ready")
	err := wait.Poll(interval, timeout, func() (done bool, err error) {
		dep, err := client.AppsV1().Deployments(namespace).Get(context.Background(), deployment, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		ready := isDeploymentReady(dep, client)
		return ready, nil
	})
	if err != nil {
		return err
	}
	logger.Debugf("%s/%s/%s is ready", deployment, namespace)

	return nil
}

func isDeploymentReady(deployment *v1.Deployment, client kubernetes.Interface) bool {
	if deployment.DeletionTimestamp != nil {
		return false
	}
	_, _, newReplicaSet, err := kctlutil.GetAllReplicaSets(deployment, client.AppsV1())
	if err != nil || newReplicaSet == nil {
		return false
	}
	if newReplicaSet.Status.ReadyReplicas < *deployment.Spec.Replicas {
		return false
	}

	return true
}
