package handlers

import (
	"net/http"
)

type GraphHandler struct{}

func NewGraphHandler() *GraphHandler {
	return &GraphHandler{}
}

func (h *GraphHandler) GetFull(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "full_graph"}, "", nil)
}

func (h *GraphHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"node": "dummy"}, "", nil)
}

func (h *GraphHandler) BlastRadius(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"radius": "calculated"}, "", nil)
}

func (h *GraphHandler) Query(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, []string{}, "", nil)
}
