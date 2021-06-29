package cluster

import "github.com/kyma-incubator/reconciler/pkg/model"

type ClusterState struct {
	Cluster       *model.ClusterEntity
	Configuration *model.ClusterConfigurationEntity
	Status        *model.ClusterStatusEntity
}
