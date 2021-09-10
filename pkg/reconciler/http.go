package reconciler

//HTTPErrorResponse is the model used for general error responses
type HTTPErrorResponse struct {
	Error string `json:"error"`
}

//HTTPMissingDependenciesResponse is the model used for missing dependency responses
type HTTPMissingDependenciesResponse struct {
	Dependencies Dependencies `json:"dependencies"`
}

type Dependencies struct {
	Required []string `json:"required"`
	Missing  []string `json:"missing"`
}

type HTTPReconciliationResponse struct {
	//mothership reconciler expects no payload in the reconciliation response at the moment
}
