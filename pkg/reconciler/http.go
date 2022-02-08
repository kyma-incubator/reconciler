package reconciler

//HTTPErrorResponse is the model used for general error responses
type HTTPErrorResponse struct {
	Error string `json:"error"`
}

type HTTPReconciliationResponse struct {
	//mothership reconciler expects no payload in the reconciliation response at the moment
}

type HTTPOccupancyUpdateRequest struct {
	Component      string `json:"component"`
	RunningWorkers int    `json:"runningWorkers"`
	PoolSize       int    `json:"poolSize"`
}
