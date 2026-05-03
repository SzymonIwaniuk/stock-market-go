package handler

import (
	"encoding/json"
	"net/http"

	"github.com/szymoniwaniuk/stock-market-go/internal/model"
	"github.com/szymoniwaniuk/stock-market-go/internal/store"
)

type LogHandler struct {
	store store.Store
}

func NewLogHandler(s store.Store) *LogHandler {
	return &LogHandler{store: s}
}

func (h *LogHandler) GetLog(w http.ResponseWriter, r *http.Request) {
	entries, err := h.store.GetLog(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	_ = json.NewEncoder(w).Encode(model.LogResponse{Log: entries})
}
