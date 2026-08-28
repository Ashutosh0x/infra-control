package handlers

import (
	"net/http"
)

// RiskHandler serves the /risk endpoints.
//
// This is a skeleton: the routes are registered but return no data.
type RiskHandler struct{}

// NewRiskHandler creates a RiskHandler.
func NewRiskHandler() *RiskHandler {
	return &RiskHandler{}
}

// Summary summarises risk.
func (h *RiskHandler) Summary(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "summary"}, "", nil)
}

// ResourceRisk handles risk.
func (h *RiskHandler) ResourceRisk(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"risk": "low"}, "", nil)
}

// Trends handles risk.
func (h *RiskHandler) Trends(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}
