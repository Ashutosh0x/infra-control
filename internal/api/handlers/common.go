// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"encoding/json"
	"log"
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

	// The status line and headers are already on the wire, so a failed encode
	// cannot be turned into an error response. Logging it is the only thing
	// left that helps, and it distinguishes a client that hung up from a
	// payload this handler cannot serialise.
	if encodeErr := json.NewEncoder(w).Encode(Response{
		Data:  data,
		Error: err,
		Meta:  meta,
	}); encodeErr != nil {
		log.Printf("write json response: %v", encodeErr)
	}
}
