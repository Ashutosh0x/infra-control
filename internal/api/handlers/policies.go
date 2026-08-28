package handlers

import (
	"net/http"
)

type PoliciesHandler struct{}

func NewPoliciesHandler() *PoliciesHandler {
	return &PoliciesHandler{}
}

func (h *PoliciesHandler) List(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

func (h *PoliciesHandler) Create(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusCreated, map[string]string{"status": "created"}, "", nil)
}

func (h *PoliciesHandler) Get(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"id": "dummy"}, "", nil)
}

func (h *PoliciesHandler) Update(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"}, "", nil)
}

func (h *PoliciesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusNoContent, nil, "", nil)
}

func (h *PoliciesHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "evaluated"}, "", nil)
}
