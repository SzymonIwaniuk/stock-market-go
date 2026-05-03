package handler

import (
	"encoding/json"
	"net/http"

	"github.com/szymoniwaniuk/stock-market-go/internal/model"
	"github.com/szymoniwaniuk/stock-market-go/internal/store"
)

type StockHandler struct {
	store store.Store
}

func NewStockHandler(s store.Store) *StockHandler {
	return &StockHandler{store: s}
}

func (h *StockHandler) GetStocks(w http.ResponseWriter, r *http.Request) {
	stocks, err := h.store.GetBankStocks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	_ = json.NewEncoder(w).Encode(model.BankState{Stocks: stocks})
}

func (h *StockHandler) SetStocks(w http.ResponseWriter, r *http.Request) {
	var req model.BankState
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.store.SetBankStocks(r.Context(), req.Stocks); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusOK)
}
