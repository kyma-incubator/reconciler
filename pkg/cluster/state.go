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

func detailsEntityToState(e *model.ActiveInventoryClusterStatusDetailsEntity) *State {
	ev := *e
	return &State{
		Cluster: &model.ClusterEntity{
			Version:    ev.ClusterID,
			RuntimeID:  ev.RuntimeID,
			Runtime:    ev.Runtime,
			Metadata:   ev.Metadata,
			Kubeconfig: ev.Kubeconfig,
			Contract:   ev.Contract,
			Deleted:    false,
			Created:    ev.ClusterCreatedAt,
		},
		Configuration: &model.ClusterConfigurationEntity{
			Version:        ev.ConfigID,
			RuntimeID:      ev.RuntimeID,
			ClusterVersion: ev.ClusterID,
			KymaVersion:    ev.KymaVersion,
			KymaProfile:    ev.KymaProfile,
			Components:     ev.Components,
			Administrators: ev.Administrators,
			Contract:       ev.Contract,
			Deleted:        false,
			Created:        ev.ConfigCreatedAt,
		},
		Status: &model.ClusterStatusEntity{
			ID:             ev.StatusID,
			RuntimeID:      ev.RuntimeID,
			ClusterVersion: ev.ClusterID,
			ConfigVersion:  ev.ConfigID,
			Status:         ev.Status,
			Deleted:        false,
			Created:        ev.StatusCreatedAt,
		},
	}
}
