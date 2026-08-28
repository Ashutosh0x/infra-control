package handlers

import (
	"net/http"
)

// GraphHandler serves the /graph endpoints.
//
// This is a skeleton: the routes are registered but return no data.
type GraphHandler struct{}

// NewGraphHandler creates a GraphHandler.
func NewGraphHandler() *GraphHandler {
	return &GraphHandler{}
}

// GetFull handles graph.
func (h *GraphHandler) GetFull(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "full_graph"}, "", nil)
}

// GetNode handles graph.
func (h *GraphHandler) GetNode(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"node": "dummy"}, "", nil)
}

// BlastRadius returns the blast radius for graph.
func (h *GraphHandler) BlastRadius(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"radius": "calculated"}, "", nil)
}

// Query queries graph.
func (h *GraphHandler) Query(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}
