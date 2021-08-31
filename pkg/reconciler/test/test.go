package test

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
)

//Deprecated: Remove this fct after all Kyma components are working without global configurations.
func NewGlobalComponentConfiguration() []reconciler.Configuration {
	return []reconciler.Configuration{
		{
			Key:   "global.ingress.domainName",
			Value: "local.kyma.dev",
		},
	}
}
