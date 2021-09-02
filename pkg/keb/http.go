package keb

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
	Error string
}
