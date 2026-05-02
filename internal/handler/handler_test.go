//go:build unit

package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/szymoniwaniuk/stock-market-go/internal/handler"
	"github.com/szymoniwaniuk/stock-market-go/internal/model"
	"github.com/szymoniwaniuk/stock-market-go/internal/store"
)

func setup(t *testing.T) (*chi.Mux, *store.RedisStore) {
	t.Helper()
	mr := miniredis.RunT(t)
	s := store.NewRedisStore(mr.Addr())

	walletH := handler.NewWalletHandler(s)
	stockH := handler.NewStockHandler(s)
	logH := handler.NewLogHandler(s)

	r := chi.NewRouter()
	r.Post("/wallets/{wallet_id}/stocks/{stock_name}", walletH.Trade)
	r.Get("/wallets/{wallet_id}", walletH.GetWallet)
	r.Get("/wallets/{wallet_id}/stocks/{stock_name}", walletH.GetWalletStock)
	r.Get("/stocks", stockH.GetStocks)
	r.Post("/stocks", stockH.SetStocks)
	r.Get("/log", logH.GetLog)
	return r, s
}

func seedBank(t *testing.T, s *store.RedisStore, stocks []model.Stock) {
	t.Helper()
	err := s.SetBankStocks(context.Background(), stocks)
	require.NoError(t, err, "failed to seed bank")
}

func doTrade(t *testing.T, r *chi.Mux, walletID, stockName, tradeType string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"type":"` + tradeType + `"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/"+walletID+"/stocks/"+stockName, bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSetAndGetStocks(t *testing.T) {
	r, _ := setup(t)

	body := `{"stocks":[{"name":"AAPL","quantity":10},{"name":"GOOG","quantity":5}]}`
	req := httptest.NewRequest(http.MethodPost, "/stocks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/stocks", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var bankState model.BankState
	require.NoError(t, json.NewDecoder(w.Body).Decode(&bankState))
	assert.Len(t, bankState.Stocks, 2)
}

func TestTrade(t *testing.T) {
	tests := []struct {
		name           string
		bankStocks     []model.Stock
		walletID       string
		stockName      string
		tradeType      string
		expectedStatus int
	}{
		{
			name:           "buy success",
			bankStocks:     []model.Stock{{Name: "AAPL", Quantity: 5}},
			walletID:       "w1",
			stockName:      "AAPL",
			tradeType:      "buy",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "buy stock not found",
			bankStocks:     nil,
			walletID:       "w1",
			stockName:      "NOPE",
			tradeType:      "buy",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "buy insufficient bank",
			bankStocks:     []model.Stock{{Name: "AAPL", Quantity: 0}},
			walletID:       "w1",
			stockName:      "AAPL",
			tradeType:      "buy",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "sell insufficient wallet",
			bankStocks:     []model.Stock{{Name: "AAPL", Quantity: 5}},
			walletID:       "w1",
			stockName:      "AAPL",
			tradeType:      "sell",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid trade type",
			bankStocks:     []model.Stock{{Name: "AAPL", Quantity: 5}},
			walletID:       "w1",
			stockName:      "AAPL",
			tradeType:      "invalid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, s := setup(t)
			if tt.bankStocks != nil {
				seedBank(t, s, tt.bankStocks)
			}

			w := doTrade(t, r, tt.walletID, tt.stockName, tt.tradeType)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestSellSuccess(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 5}})

	w := doTrade(t, r, "w1", "AAPL", "buy")
	require.Equal(t, http.StatusOK, w.Code)

	w = doTrade(t, r, "w1", "AAPL", "sell")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBuyDecrementsBank(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 3}})

	w := doTrade(t, r, "w1", "AAPL", "buy")
	require.Equal(t, http.StatusOK, w.Code)

	req := httptest.NewRequest(http.MethodGet, "/stocks", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var bank model.BankState
	require.NoError(t, json.NewDecoder(w.Body).Decode(&bank))
	require.Len(t, bank.Stocks, 1)
	assert.Equal(t, 2, bank.Stocks[0].Quantity)
}

func TestSellIncrementsBank(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 3}})

	w := doTrade(t, r, "w1", "AAPL", "buy")
	require.Equal(t, http.StatusOK, w.Code)

	w = doTrade(t, r, "w1", "AAPL", "sell")
	require.Equal(t, http.StatusOK, w.Code)

	req := httptest.NewRequest(http.MethodGet, "/stocks", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var bank model.BankState
	require.NoError(t, json.NewDecoder(w.Body).Decode(&bank))
	require.Len(t, bank.Stocks, 1)
	assert.Equal(t, 3, bank.Stocks[0].Quantity)
}

func TestGetWallet(t *testing.T) {
	tests := []struct {
		name          string
		walletID      string
		buyCount      int
		expectedID    string
		expectedStock int
	}{
		{
			name:          "wallet with stocks",
			walletID:      "w1",
			buyCount:      3,
			expectedID:    "w1",
			expectedStock: 3,
		},
		{
			name:          "non-existent wallet",
			walletID:      "nonexistent",
			buyCount:      0,
			expectedID:    "nonexistent",
			expectedStock: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, s := setup(t)
			seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 5}})

			for i := 0; i < tt.buyCount; i++ {
				w := doTrade(t, r, tt.walletID, "AAPL", "buy")
				require.Equal(t, http.StatusOK, w.Code)
			}

			req := httptest.NewRequest(http.MethodGet, "/wallets/"+tt.walletID, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var wallet model.WalletResponse
			require.NoError(t, json.NewDecoder(w.Body).Decode(&wallet))
			assert.Equal(t, tt.expectedID, wallet.ID)

			if tt.expectedStock == 0 {
				assert.Empty(t, wallet.Stocks)
			} else {
				require.Len(t, wallet.Stocks, 1)
				assert.Equal(t, tt.expectedStock, wallet.Stocks[0].Quantity)
			}
		})
	}
}

func TestGetWalletStock(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 5}})

	w := doTrade(t, r, "w1", "AAPL", "buy")
	require.Equal(t, http.StatusOK, w.Code)

	req := httptest.NewRequest(http.MethodGet, "/wallets/w1/stocks/AAPL", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "1", w.Body.String())
}

func TestAuditLog(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 5}})

	w := doTrade(t, r, "w1", "AAPL", "buy")
	require.Equal(t, http.StatusOK, w.Code)
	w = doTrade(t, r, "w1", "AAPL", "sell")
	require.Equal(t, http.StatusOK, w.Code)

	req := httptest.NewRequest(http.MethodGet, "/log", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var logResp model.LogResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&logResp))
	require.Len(t, logResp.Log, 2)
	assert.Equal(t, "buy", logResp.Log[0].Type)
	assert.Equal(t, "sell", logResp.Log[1].Type)
}

func TestFailedOperationsNotLogged(t *testing.T) {
	r, s := setup(t)

	doTrade(t, r, "w1", "NOPE", "buy")

	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 0}})
	doTrade(t, r, "w1", "AAPL", "buy")

	req := httptest.NewRequest(http.MethodGet, "/log", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var logResp model.LogResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&logResp))
	assert.Empty(t, logResp.Log)
}
