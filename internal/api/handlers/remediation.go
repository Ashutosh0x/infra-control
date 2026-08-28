package handlers

import (
	"net/http"
)

type RemediationHandler struct{}

func NewRemediationHandler() *RemediationHandler {
	return &RemediationHandler{}
}

func (h *RemediationHandler) List(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

func (h *RemediationHandler) Get(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"id": "dummy"}, "", nil)
}

func (h *RemediationHandler) Approve(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "approved"}, "", nil)
}

func (h *RemediationHandler) Reject(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"}, "", nil)
}

func (h *RemediationHandler) Execute(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "executing"}, "", nil)
}
