package keb

import "time"

const (
	ClusterStatusPending     ClusterStatus = "reconcile_pending"
	ClusterStatusReady       ClusterStatus = "ready"
	ClusterStatusError       ClusterStatus = "error"
	ClusterStatusReconciling ClusterStatus = "reconciling"
)

type ClusterStatus string

type HTTPClusterResponse struct {
	Cluster              string        `json:"cluster"`
	ClusterVersion       int64         `json:"clusterVersion"`
	ConfigurationVersion int64         `json:"configurationVersion"`
	Status               ClusterStatus `json:"status"`
	StatusURL            string        `json:"statusUrl"`
}

//HTTPErrorResponse is the model used for general error responses
type HTTPErrorResponse struct {
	Error string `json:"error"`
}

type HTTPClusterStatusResponse struct {
	StatusChanges []*StatusChange
}

type StatusChange struct {
	Started  time.Time     `json:"started"`
	Duration time.Duration `json:"duration"`
	Status   ClusterStatus `json:"status"`
}
