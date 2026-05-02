package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/szymoniwaniuk/stock-market-go/internal/model"
	"github.com/szymoniwaniuk/stock-market-go/internal/store"
)

type WalletHandler struct {
	store store.Store
}

func NewWalletHandler(s store.Store) *WalletHandler {
	return &WalletHandler{store: s}
}

func (h *WalletHandler) Trade(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "wallet_id")
	stockName := chi.URLParam(r, "stock_name")

	var req model.TradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type != "buy" && req.Type != "sell" {
		http.Error(w, "type must be 'buy' or 'sell'", http.StatusBadRequest)
		return
	}

	err := h.store.ExecuteTrade(r.Context(), walletID, stockName, req.Type)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrStockNotFound):
			http.Error(w, "stock not found", http.StatusNotFound)
		case errors.Is(err, store.ErrInsufficientBank):
			http.Error(w, "no stock available in bank", http.StatusBadRequest)
		case errors.Is(err, store.ErrInsufficientWallet):
			http.Error(w, "no stock available in wallet", http.StatusBadRequest)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WalletHandler) GetWallet(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "wallet_id")

	wallet, err := h.store.GetWallet(r.Context(), walletID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wallet)
}

func (h *WalletHandler) GetWalletStock(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "wallet_id")
	stockName := chi.URLParam(r, "stock_name")

	qty, err := h.store.GetWalletStock(r.Context(), walletID, stockName)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, qty)
}
