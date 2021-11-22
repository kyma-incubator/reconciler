package converters

import (
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

func ConvertConfig(entity model.ClusterConfigurationEntity) keb.KymaConfig {
	components := make([]keb.Component, len(entity.Components))
	for i, component := range entity.Components {
		components[i] = *component
	}

	out := keb.KymaConfig{
		Administrators: entity.Administrators,
		Components:     components,
		Profile:        entity.KymaProfile,
		Version:        entity.KymaVersion,
	}
	return out
}
