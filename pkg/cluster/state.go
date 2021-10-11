package cluster

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/model"
)

type State struct {
	Cluster       *model.ClusterEntity
	Configuration *model.ClusterConfigurationEntity
	Status        *model.ClusterStatusEntity
}

func (s *State) String() string {
	return fmt.Sprintf("State [RuntimeID=%s,ClusterVersion=%d,ConfigVersion=%d,Status=%s]",
		s.Cluster.RuntimeID, s.Cluster.Version, s.Configuration.Version, s.Status.Status)
}
