// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"encoding/json"
	"net/http"
)

// Response Envelope
type Response struct {
	Data  any               `json:"data,omitempty"`
	Error string            `json:"error,omitempty"`
	Meta  map[string]string `json:"meta,omitempty"`
}

// WriteJSON sends a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, data any, err string, meta map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Data:  data,
		Error: err,
		Meta:  meta,
	})
}
