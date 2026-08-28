package handlers

import (
	"net/http"
)

// ComplianceHandler serves the /compliance endpoints.
//
// This is a skeleton: the routes are registered but return no data.
type ComplianceHandler struct{}

// NewComplianceHandler creates a ComplianceHandler.
func NewComplianceHandler() *ComplianceHandler {
	return &ComplianceHandler{}
}

// Frameworks handles compliance.
func (h *ComplianceHandler) Frameworks(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

// Status returns the status of compliance.
func (h *ComplianceHandler) Status(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "compliant"}, "", nil)
}

// Report reports on compliance.
func (h *ComplianceHandler) Report(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"report": "generated"}, "", nil)
}
