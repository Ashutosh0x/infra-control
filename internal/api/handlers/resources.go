package handlers

import (
	"net/http"
)

// ResourcesHandler serves the /resources endpoints.
//
// This is a skeleton: the routes are registered but return no data.
type ResourcesHandler struct{}

// NewResourcesHandler creates a ResourcesHandler.
func NewResourcesHandler() *ResourcesHandler {
	return &ResourcesHandler{}
}

// List lists resources.
func (h *ResourcesHandler) List(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

// Get returns resources.
func (h *ResourcesHandler) Get(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"id": "dummy"}, "", nil)
}

// Discover triggers discovery of resources.
func (h *ResourcesHandler) Discover(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "discovering"}, "", nil)
}

// Risk returns the risk score for resources.
func (h *ResourcesHandler) Risk(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"risk": "low"}, "", nil)
}
