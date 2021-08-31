package test

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
)

//NewGlobalComponentConfiguration returns default configuration values required by several Kyma components.
//Deprecated: Remove this fct after all Kyma components are working without global configurations.
//nolint:SA1019 - don't report this deprecated function during linting
func NewGlobalComponentConfiguration() []reconciler.Configuration {
	return []reconciler.Configuration{
		{
			Key:   "global.ingress.domainName",
			Value: "local.kyma.dev",
		},
	}
}
