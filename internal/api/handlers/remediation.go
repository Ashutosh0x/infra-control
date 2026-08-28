package handlers

import (
	"net/http"
)

// RemediationHandler serves the /remediation endpoints.
//
// This is a skeleton: the routes are registered but return no data.
type RemediationHandler struct{}

// NewRemediationHandler creates a RemediationHandler.
func NewRemediationHandler() *RemediationHandler {
	return &RemediationHandler{}
}

// List lists remediation.
func (h *RemediationHandler) List(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

// Get returns remediation.
func (h *RemediationHandler) Get(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"id": "dummy"}, "", nil)
}

// Approve approves remediation.
func (h *RemediationHandler) Approve(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "approved"}, "", nil)
}

// Reject rejects remediation.
func (h *RemediationHandler) Reject(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"}, "", nil)
}

// Execute executes remediation.
func (h *RemediationHandler) Execute(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "executing"}, "", nil)
}
