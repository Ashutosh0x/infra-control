package handlers

import (
	"net/http"
)

// DriftHandler serves the /drift endpoints.
//
// This is a skeleton: the routes are registered but return no data.
type DriftHandler struct{}

// NewDriftHandler creates a DriftHandler.
func NewDriftHandler() *DriftHandler {
	return &DriftHandler{}
}

// List lists drift.
func (h *DriftHandler) List(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

// Get returns drift.
func (h *DriftHandler) Get(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"id": "dummy"}, "", nil)
}

// Scan scans drift.
func (h *DriftHandler) Scan(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "scanning"}, "", nil)
}

// Resolve resolves drift.
func (h *DriftHandler) Resolve(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "resolved"}, "", nil)
}
