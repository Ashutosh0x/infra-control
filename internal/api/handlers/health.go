package handlers

import (
	"net/http"
)

// HealthHandler handles health check endpoints.
// HealthHandler serves the /health endpoints.
//
// This is a skeleton: the routes are registered but return no data.
type HealthHandler struct{}

// NewHealthHandler creates a new HealthHandler.
// NewHealthHandler creates a HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Liveness returns 200 OK if the server is running.
// Liveness handles health.
func (h *HealthHandler) Liveness(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"}, "", nil)
}

// Readiness returns 200 OK if the server is ready to accept traffic.
// Readiness handles health.
func (h *HealthHandler) Readiness(w http.ResponseWriter, _ *http.Request) {
	// TODO: check DB, cache, event bus
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"}, "", nil)
}

// Version returns version information.
// Version handles health.
func (h *HealthHandler) Version(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"version": "dev"}, "", nil)
}
