package reconciler

//HTTPErrorResponse is the model used for general error responses
type HTTPErrorResponse struct {
	Error string `json:"error"`
}

type HTTPReconciliationResponse struct {
	PoolID   string
	PoolSize int
}
