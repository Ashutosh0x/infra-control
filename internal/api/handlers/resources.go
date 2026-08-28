package handlers

import (
	"net/http"
)

type ResourcesHandler struct{}

func NewResourcesHandler() *ResourcesHandler {
	return &ResourcesHandler{}
}

func (h *ResourcesHandler) List(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

func (h *ResourcesHandler) Get(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"id": "dummy"}, "", nil)
}

func (h *ResourcesHandler) Discover(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "discovering"}, "", nil)
}

func (h *ResourcesHandler) Risk(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"risk": "low"}, "", nil)
}
