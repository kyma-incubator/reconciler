package model

import (
	"fmt"
)

type Status string

const (
	ReconcilePending Status = "reconcile_pending"
	ReconcileFailed  Status = "reconcile_failed"
	Reconciling      Status = "reconciling"
	Error            Status = "error"
	Ready            Status = "ready"
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
	case Error:
		clusterStatus.Status = Error
		clusterStatus.ID = 0
	case Ready:
		clusterStatus.Status = Ready
		clusterStatus.ID = 1
	case ReconcilePending:
		clusterStatus.Status = ReconcilePending
		clusterStatus.ID = 2
	case Reconciling:
		clusterStatus.Status = Reconciling
		clusterStatus.ID = 3
	case ReconcileFailed:
		clusterStatus.Status = ReconcileFailed
		clusterStatus.ID = 4
	default:
		return clusterStatus, fmt.Errorf("ClusterStatus '%s' is unknown", status)
	}
	return clusterStatus, nil
}
