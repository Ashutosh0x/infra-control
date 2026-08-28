package handlers

import (
	"net/http"
)

type RiskHandler struct{}

func NewRiskHandler() *RiskHandler {
	return &RiskHandler{}
}

func (h *RiskHandler) Summary(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "summary"}, "", nil)
}

func (h *RiskHandler) ResourceRisk(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"risk": "low"}, "", nil)
}

func (h *RiskHandler) Trends(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}
