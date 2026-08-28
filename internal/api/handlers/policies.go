package handlers

import (
	"net/http"
)

// PoliciesHandler serves the /policies endpoints.
//
// This is a skeleton: the routes are registered but return no data.
type PoliciesHandler struct{}

// NewPoliciesHandler creates a PoliciesHandler.
func NewPoliciesHandler() *PoliciesHandler {
	return &PoliciesHandler{}
}

// List lists policies.
func (h *PoliciesHandler) List(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

// Create creates policies.
func (h *PoliciesHandler) Create(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusCreated, map[string]string{"status": "created"}, "", nil)
}

// Get returns policies.
func (h *PoliciesHandler) Get(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"id": "dummy"}, "", nil)
}

// Update updates policies.
func (h *PoliciesHandler) Update(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"}, "", nil)
}

// Delete deletes policies.
func (h *PoliciesHandler) Delete(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusNoContent, nil, "", nil)
}

// Evaluate evaluates policies.
func (h *PoliciesHandler) Evaluate(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "evaluated"}, "", nil)
}
