package connectivityproxy

import (
	"fmt"

	k8s "k8s.io/client-go/kubernetes"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	reconcilerK8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	ReconcilerName       = "connectivity-proxy"
	registryConfigPrefix = "registry"
	istioConfigPrefix    = "istio"
)

type CopyFactory func(configs map[string]interface{}, inClusterClientSet, targetClientSet k8s.Interface) *SecretCopy

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	reconciler.
		WithReconcileAction(&CustomAction{
			Name:   "action",
			Loader: &K8sLoader{},
			Commands: &CommandActions{
				clientSetFactory: reconcilerK8s.NewInClusterClientSet,
				targetClientSetFactory: func(context *service.ActionContext) (k8s.Interface, error) {
					return context.KubeClient.Clientset()
				},
				install: service.NewInstall(log),
				copyFactory: []CopyFactory{
					registrySecretCopy,
					istioSecretCopy,
				},
			},
		})
}

func registrySecretCopy(configs map[string]interface{}, inClusterClientSet, targetClientSet k8s.Interface) *SecretCopy {
	return &SecretCopy{
		Namespace:       fmt.Sprintf("%v", configs[registryConfigPrefix+".secret.to.namespace"]),
		Name:            fmt.Sprintf("%v", configs[registryConfigPrefix+".secret.name"]),
		targetClientSet: targetClientSet,
		from: &FromSecret{
			Name:      fmt.Sprintf("%v", configs[registryConfigPrefix+".secret.name"]),
			Namespace: fmt.Sprintf("%v", configs[registryConfigPrefix+".secret.from.namespace"]),
			inCluster: inClusterClientSet,
		},
	}
}

func istioSecretCopy(configs map[string]interface{}, _, targetClientSet k8s.Interface) *SecretCopy {
	return &SecretCopy{
		Namespace:       fmt.Sprintf("%v", configs[istioConfigPrefix+".secret.namespace"]),
		Name:            fmt.Sprintf("%v", configs[istioConfigPrefix+".secret.name"]),
		targetClientSet: targetClientSet,
		from: &FromURL{
			URL: fmt.Sprintf("%v", configs[istioConfigPrefix+".secret.url"]),
			Key: fmt.Sprintf("%v", configs[istioConfigPrefix+".secret.key"]),
		},
	}
}
