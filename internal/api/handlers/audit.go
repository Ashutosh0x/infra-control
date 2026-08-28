package handlers

import (
	"net/http"
)

// AuditHandler serves the /audit endpoints.
//
// This is a skeleton: the routes are registered but return no data.
type AuditHandler struct{}

// NewAuditHandler creates a AuditHandler.
func NewAuditHandler() *AuditHandler {
	return &AuditHandler{}
}

// List lists audit.
func (h *AuditHandler) List(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

// Get returns audit.
func (h *AuditHandler) Get(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"id": "dummy"}, "", nil)
}

// Export exports audit.
func (h *AuditHandler) Export(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "exported"}, "", nil)
}
