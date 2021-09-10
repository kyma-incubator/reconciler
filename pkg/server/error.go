package server

import (
	"encoding/json"
	"net/http"
)

func SendHTTPError(w http.ResponseWriter, httpCode int, resp interface{}) {
	w.WriteHeader(httpCode)
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, "Failed to encode response payload to JSON", http.StatusInternalServerError)
	}
}
