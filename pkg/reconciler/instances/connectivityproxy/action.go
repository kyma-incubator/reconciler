package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

type CustomAction struct {
	name string
}

const (
	registryConfigPrefix = "registry"
	istioConfigPrefix    = "istio"
)

func (a *CustomAction) Run(version, _ string, _ []reconciler.Configuration, context *service.ActionContext) error {
	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, version)

	// registry
	copy := NewFromSecret(registryConfigPrefix, context.ClientSet, context.InClusterClientSet, context.ConfigsMap)
	err := copy.Transfer()
	if err != nil {
		return err
	}

	// url
	copy = NewFromURL(istioConfigPrefix, context.ClientSet, context.ConfigsMap)
	err = copy.Transfer()
	if err != nil {
		return err
	}

	return nil
}
