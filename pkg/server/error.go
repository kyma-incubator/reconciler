package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func SendHTTPError(w http.ResponseWriter, httpCode int, resp interface{}) {
	w.WriteHeader(httpCode)
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, fmt.Sprintf("Failed to encode response payload to JSON: %s", err), http.StatusInternalServerError)
	}
}
