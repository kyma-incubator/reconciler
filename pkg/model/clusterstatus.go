package model

import (
	"fmt"
)

type Status string

const (
	ClusterStatusReconcilePending Status = "reconcile_pending"
	ClusterStatusReconcileFailed  Status = "reconcile_failed"
	ClusterStatusReconciling      Status = "reconciling"
	ClusterStatusError            Status = "error"
	ClusterStatusReady            Status = "ready"
)

type ClusterStatus struct {
	ID     float64 //required for monitoring metrics, has to be unique!
	Status Status
}

func (s *ClusterStatus) String() string {
	return string(s.Status)
}

func NewClusterStatus(status Status) (*ClusterStatus, error) {
	clusterStatus := &ClusterStatus{}
	switch status {
	case ClusterStatusError:
		clusterStatus.Status = ClusterStatusError
		clusterStatus.ID = 0
	case ClusterStatusReady:
		clusterStatus.Status = ClusterStatusReady
		clusterStatus.ID = 1
	case ClusterStatusReconcilePending:
		clusterStatus.Status = ClusterStatusReconcilePending
		clusterStatus.ID = 2
	case ClusterStatusReconciling:
		clusterStatus.Status = ClusterStatusReconciling
		clusterStatus.ID = 3
	case ClusterStatusReconcileFailed:
		clusterStatus.Status = ClusterStatusReconcileFailed
		clusterStatus.ID = 4
	default:
		return clusterStatus, fmt.Errorf("ClusterStatus '%s' is unknown", status)
	}
	return clusterStatus, nil
}
