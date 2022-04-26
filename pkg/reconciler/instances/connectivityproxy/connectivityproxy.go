package connectivityproxy

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	reconcilerK8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	ReconcilerName    = "connectivity-proxy"
	istioConfigPrefix = "istio"
)

type CopyFactory func(task *reconciler.Task, inClusterClientSet, targetClientSet k8s.Interface) *SecretCopy

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconcilerInstance, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	action := CustomAction{
		Name:   "action",
		Loader: &K8sLoader{},
		Commands: &CommandActions{
			clientSetFactory: reconcilerK8s.NewInClusterClientSet,
			targetClientSetFactory: func(context *service.ActionContext) (k8s.Interface, error) {
				return context.KubeClient.Clientset()
			},
			install: service.NewInstall(log),
			copyFactory: []CopyFactory{
				istioSecretCopy,
			},
		},
	}
	reconcilerInstance.
		WithDeleteAction(&action).
		WithReconcileAction(&action)
}

func istioSecretCopy(task *reconciler.Task, _, targetClientSet k8s.Interface) *SecretCopy {
	configs := task.Configuration

	istioNamespace := configs[istioConfigPrefix+".secret.namespace"]
	if istioNamespace == nil || istioNamespace == "" {
		istioNamespace = "istio-system"
	}
	istioSecretKey := configs[istioConfigPrefix+".secret.key"]
	if istioSecretKey == nil || istioSecretKey == "" {
		istioSecretKey = "cacert"
	}

	return &SecretCopy{
		Namespace:       fmt.Sprintf("%v", istioNamespace),
		Name:            fmt.Sprintf("%v", configs[istioConfigPrefix+".secret.name"]),
		targetClientSet: targetClientSet,
		from: &FromURL{
			URL: fmt.Sprintf("%v%v",
				configs[BindingKey+"url"],
				configs[BindingKey+"CAs_path"]),
			Key: fmt.Sprintf("%v", istioSecretKey),
		},
	}
}
