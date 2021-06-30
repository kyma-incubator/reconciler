package model

import (
	"fmt"
	"strings"
)

const (
	ReconcilePending ClusterStatus = "reconcile_pending"
	ReconcileFailed  ClusterStatus = "reconcile_failed"
	Reconciling      ClusterStatus = "reconciling"
	Error            ClusterStatus = "error"
	Ready            ClusterStatus = "ready"
)

type ClusterStatus string

func NewClusterStatus(clusterStatus string) (ClusterStatus, error) {
	switch strings.ToLower(clusterStatus) {
	case string(ReconcilePending):
		return ReconcilePending, nil
	case string(ReconcileFailed):
		return ReconcileFailed, nil
	case string(Ready):
		return Ready, nil
	case string(Reconciling):
		return Reconciling, nil
	case string(Error):
		return Error, nil
	default:
		return "", fmt.Errorf("ClusterStatus '%s' is unknown", clusterStatus)
	}
}
