package handlers

import (
	"net/http"
)

type ComplianceHandler struct{}

func NewComplianceHandler() *ComplianceHandler {
	return &ComplianceHandler{}
}

func (h *ComplianceHandler) Frameworks(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

func (h *ComplianceHandler) Status(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "compliant"}, "", nil)
}

func (h *ComplianceHandler) Report(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"report": "generated"}, "", nil)
}
