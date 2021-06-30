package cluster

import "github.com/kyma-incubator/reconciler/pkg/model"

type State struct {
	Cluster       *model.ClusterEntity
	Configuration *model.ClusterConfigurationEntity
	Status        *model.ClusterStatusEntity
}
