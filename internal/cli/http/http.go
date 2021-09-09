package http

import (
	"encoding/json"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"net/http"
)

func SendHTTPError(w http.ResponseWriter, httpCode int, response interface{}) {
	if err, ok := response.(error); ok { //convert to error response
		response = reconciler.HTTPErrorResponse{
			Error: err.Error(),
		}
	}
	w.WriteHeader(httpCode)
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		//send error response
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, "Failed to encode response payload to JSON", http.StatusInternalServerError)
	}
}
