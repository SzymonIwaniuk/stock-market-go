package handler

import (
	"net/http"

	"github.com/szymoniwaniuk/stock-market-go/internal/store"
)

type ResetHandler struct {
	store store.Store
}

func NewResetHandler(s store.Store) *ResetHandler {
	return &ResetHandler{store: s}
}

func (h *ResetHandler) Reset(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Flush(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusOK)
}
