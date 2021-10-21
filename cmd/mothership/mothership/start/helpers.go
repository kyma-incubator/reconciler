package cmd

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

func components(cfg model.ClusterConfigurationEntity) []keb.Component {
	componentsLen := len(cfg.Components)
	components := make([]keb.Component, componentsLen)

	for i := 0; i < componentsLen; i++ {
		component := cfg.Components[i]
		if component != nil {
			components[i] = *component
			continue
		}
	}
	return components
}

func clusterMetadata(runtimeID string, lastState *cluster.State) *keb.Cluster {
	// sanitize metadata
	var metadata keb.Metadata
	var hasMetadata = lastState.Cluster != nil && lastState.Cluster.Metadata != nil
	if hasMetadata {
		metadata = *lastState.Cluster.Metadata
	}

	// sanitize runtime input
	var runtimeInput keb.RuntimeInput
	var hasRuntimeInput = lastState.Cluster != nil && lastState.Cluster.Runtime != nil
	if hasRuntimeInput {
		runtimeInput = *lastState.Cluster.Runtime
	}

	var configuration model.ClusterConfigurationEntity
	if lastState.Configuration == nil {
		configuration = model.ClusterConfigurationEntity{}
	}

	// sanitize components
	components := components(configuration)

	return &keb.Cluster{
		RuntimeID:    runtimeID,
		Metadata:     metadata,
		RuntimeInput: runtimeInput,
		Kubeconfig:   lastState.Cluster.Kubeconfig,
		KymaConfig: keb.KymaConfig{
			Administrators: lastState.Configuration.Administrators,
			Components:     components,
			Profile:        lastState.Configuration.KymaProfile,
			Version:        lastState.Configuration.KymaVersion,
		},
	}
}
