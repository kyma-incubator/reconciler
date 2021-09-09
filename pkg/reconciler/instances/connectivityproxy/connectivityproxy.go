package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	service "github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	ReconcilerName       = "connectivity-proxy"
	registryConfigPrefix = "registry"
	istioConfigPrefix    = "istio"
)

type CopyFactory func(context *service.ActionContext) *SecretCopy

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log, err := logger.NewLogger(false)
	if err != nil {
		panic(err)
	}

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	reconciler.
		WithPreReconcileAction(&CustomAction{
			name: "pre-action",
			copyFactory: []CopyFactory{
				registrySecretCopy,
				istioSecretCopy,
			},
		})
}

func registrySecretCopy(context *service.ActionContext) *SecretCopy {
	configs := context.ConfigsMap

	return &SecretCopy{
		Namespace:       configs[registryConfigPrefix+".secret.to.namespace"],
		Name:            configs[registryConfigPrefix+".secret.name"],
		targetClientSet: context.ClientSet,
		from: &FromSecret{
			Name:      configs[registryConfigPrefix+".secret.name"],
			Namespace: configs[registryConfigPrefix+".secret.from.namespace"],
			inCluster: context.InClusterClientSet,
		},
	}
}

func istioSecretCopy(context *service.ActionContext) *SecretCopy {
	configs := context.ConfigsMap

	return &SecretCopy{
		Namespace:       configs[istioConfigPrefix+".secret.namespace"],
		Name:            configs[istioConfigPrefix+".secret.name"],
		targetClientSet: context.ClientSet,
		from: &FromURL{
			URL: configs[istioConfigPrefix+".secret.url"],
			Key: configs[istioConfigPrefix+".secret.key"],
		},
	}
}
