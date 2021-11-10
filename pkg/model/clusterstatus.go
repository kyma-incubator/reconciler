package model

import (
	"fmt"
)

type Status string

const (
	ClusterStatusDeletePending     Status = "delete_pending"
	ClusterStatusDeleting          Status = "deleting"
	ClusterStatusDeleteError       Status = "delete_error"
	ClusterStatusDeleted           Status = "deleted"
	ClusterStatusReconcilePending  Status = "reconcile_pending"
	ClusterStatusReconcileDisabled Status = "reconcile_disabled"
	ClusterStatusReconciling       Status = "reconciling"
	ClusterStatusReconcileError    Status = "error"
	ClusterStatusReady             Status = "ready"
)

func (s Status) IsDeletion() bool {
	return s == ClusterStatusDeletePending || s == ClusterStatusDeleting
}
func (s Status) IsDeleteCandidate() bool {
	return s == ClusterStatusDeletePending
}

func (s Status) IsReconcileCandidate() bool {
	return s == ClusterStatusReconcilePending || s == ClusterStatusReady
}

func (s Status) IsFinal() bool {
	return s == ClusterStatusReady || s == ClusterStatusReconcileError || s == ClusterStatusDeleted || s == ClusterStatusDeleteError
}

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
	case ClusterStatusReconcileError:
		clusterStatus.ID = 0
	case ClusterStatusReady:
		clusterStatus.ID = 1
	case ClusterStatusReconcilePending:
		clusterStatus.ID = 2
	case ClusterStatusReconciling:
		clusterStatus.ID = 3
	case ClusterStatusReconcileDisabled:
		clusterStatus.ID = 4
	case ClusterStatusDeletePending:
		clusterStatus.ID = 5
	case ClusterStatusDeleting:
		clusterStatus.ID = 6
	case ClusterStatusDeleteError:
		clusterStatus.ID = 7
	case ClusterStatusDeleted:
		clusterStatus.ID = 8
	default:
		return clusterStatus, fmt.Errorf("ClusterStatus '%s' is unknown", status)
	}
	clusterStatus.Status = status
	return clusterStatus, nil
}
