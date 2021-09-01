package keb

type HTTPClusterResponse struct {
	Cluster              string `json:"cluster"`
	ClusterVersion       int64  `json:"clusterVersion"`
	ConfigurationVersion int64  `json:"configurationVersion"`
	Status               string `json:"status"`
	StatusURL            string `json:"statusUrl"`
}
