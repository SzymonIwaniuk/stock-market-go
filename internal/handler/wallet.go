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
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Type != "buy" && req.Type != "sell" {
		writeError(w, http.StatusBadRequest, "type must be 'buy' or 'sell'")
		return
	}

	err := h.store.ExecuteTrade(r.Context(), walletID, stockName, req.Type)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrStockNotFound):
			writeError(w, http.StatusNotFound, "stock not found")
		case errors.Is(err, store.ErrInsufficientBank):
			writeError(w, http.StatusBadRequest, "no stock available in bank")
		case errors.Is(err, store.ErrInsufficientWallet):
			writeError(w, http.StatusBadRequest, "no stock available in wallet")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WalletHandler) GetWallet(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "wallet_id")

	wallet, err := h.store.GetWallet(r.Context(), walletID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	_ = json.NewEncoder(w).Encode(wallet)
}

func (h *WalletHandler) GetWalletStock(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "wallet_id")
	stockName := chi.URLParam(r, "stock_name")

	qty, err := h.store.GetWalletStock(r.Context(), walletID, stockName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	_, _ = fmt.Fprint(w, qty)
}
