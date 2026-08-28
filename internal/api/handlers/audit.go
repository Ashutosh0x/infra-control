package handlers

import (
	"net/http"
)

type AuditHandler struct{}

func NewAuditHandler() *AuditHandler {
	return &AuditHandler{}
}

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

func (h *AuditHandler) Get(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"id": "dummy"}, "", nil)
}

func (h *AuditHandler) Export(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "exported"}, "", nil)
}
