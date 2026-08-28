package handlers

import (
	"net/http"
)

type DriftHandler struct{}

func NewDriftHandler() *DriftHandler {
	return &DriftHandler{}
}

func (h *DriftHandler) List(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}

func (h *DriftHandler) Get(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"id": "dummy"}, "", nil)
}

func (h *DriftHandler) Scan(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "scanning"}, "", nil)
}

func (h *DriftHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "resolved"}, "", nil)
}
