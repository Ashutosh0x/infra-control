package handlers

import (
	"net/http"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct{}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Liveness returns 200 OK if the server is running.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"}, "", nil)
}

// Readiness returns 200 OK if the server is ready to accept traffic.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	// TODO: check DB, cache, event bus
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"}, "", nil)
}

// Version returns version information.
func (h *HealthHandler) Version(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"version": "dev"}, "", nil)
}
