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
	if err := s.SetBankStocks(context.Background(), stocks); err != nil {
		t.Fatalf("failed to seed bank: %v", err)
	}
}

func TestSetAndGetStocks(t *testing.T) {
	r, _ := setup(t)

	body := `{"stocks":[{"name":"AAPL","quantity":10},{"name":"GOOG","quantity":5}]}`
	req := httptest.NewRequest(http.MethodPost, "/stocks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /stocks: expected 200, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/stocks", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /stocks: expected 200, got %d", w.Code)
	}

	var bankState model.BankState
	json.NewDecoder(w.Body).Decode(&bankState)

	if len(bankState.Stocks) != 2 {
		t.Fatalf("expected 2 stocks, got %d", len(bankState.Stocks))
	}
}

func TestBuySuccess(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 5}})

	body := `{"type":"buy"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/wallets/w1/stocks/AAPL", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Body.String() != "1" {
		t.Fatalf("expected wallet stock qty 1, got %s", w.Body.String())
	}
}

func TestBuyStockNotFound(t *testing.T) {
	r, _ := setup(t)

	body := `{"type":"buy"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/NOPE", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestBuyInsufficientBank(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 0}})

	body := `{"type":"buy"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSellSuccess(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 5}})

	body := `{"type":"buy"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("buy failed: %d", w.Code)
	}

	body = `{"type":"sell"}`
	req = httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSellInsufficientWallet(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 5}})

	body := `{"type":"sell"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetWallet(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 5}})

	for i := 0; i < 3; i++ {
		body := `{"type":"buy"}`
		req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("buy %d failed: %d", i, w.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/wallets/w1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var wallet model.WalletResponse
	json.NewDecoder(w.Body).Decode(&wallet)

	if wallet.ID != "w1" {
		t.Fatalf("expected wallet id w1, got %s", wallet.ID)
	}
	if len(wallet.Stocks) != 1 || wallet.Stocks[0].Quantity != 3 {
		t.Fatalf("expected 1 stock with qty 3, got %+v", wallet.Stocks)
	}
}

func TestAuditLog(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 5}})

	body := `{"type":"buy"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body = `{"type":"sell"}`
	req = httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	req = httptest.NewRequest(http.MethodGet, "/log", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var logResp model.LogResponse
	json.NewDecoder(w.Body).Decode(&logResp)

	if len(logResp.Log) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logResp.Log))
	}
	if logResp.Log[0].Type != "buy" {
		t.Fatalf("expected first entry type buy, got %s", logResp.Log[0].Type)
	}
	if logResp.Log[1].Type != "sell" {
		t.Fatalf("expected second entry type sell, got %s", logResp.Log[1].Type)
	}
}

func TestInvalidTradeType(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 5}})

	body := `{"type":"invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetWalletNonExistent(t *testing.T) {
	r, _ := setup(t)

	req := httptest.NewRequest(http.MethodGet, "/wallets/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var wallet model.WalletResponse
	json.NewDecoder(w.Body).Decode(&wallet)

	if wallet.ID != "nonexistent" {
		t.Fatalf("expected id nonexistent, got %s", wallet.ID)
	}
	if len(wallet.Stocks) != 0 {
		t.Fatalf("expected empty stocks, got %+v", wallet.Stocks)
	}
}

func TestFailedOperationsNotLogged(t *testing.T) {
	r, s := setup(t)

	// Buy a non-existent stock (404)
	body := `{"type":"buy"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/NOPE", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Buy with zero bank quantity (400)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 0}})
	body = `{"type":"buy"}`
	req = httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	req = httptest.NewRequest(http.MethodGet, "/log", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var logResp model.LogResponse
	json.NewDecoder(w.Body).Decode(&logResp)

	if len(logResp.Log) != 0 {
		t.Fatalf("expected 0 log entries for failed ops, got %d", len(logResp.Log))
	}
}

func TestBuyDecrementsBank(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 3}})

	body := `{"type":"buy"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	req = httptest.NewRequest(http.MethodGet, "/stocks", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var bank model.BankState
	json.NewDecoder(w.Body).Decode(&bank)

	if len(bank.Stocks) != 1 || bank.Stocks[0].Quantity != 2 {
		t.Fatalf("expected bank AAPL qty 2, got %+v", bank.Stocks)
	}
}

func TestSellIncrementsBank(t *testing.T) {
	r, s := setup(t)
	seedBank(t, s, []model.Stock{{Name: "AAPL", Quantity: 3}})

	// Buy first
	body := `{"type":"buy"}`
	req := httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Sell back
	body = `{"type":"sell"}`
	req = httptest.NewRequest(http.MethodPost, "/wallets/w1/stocks/AAPL", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	req = httptest.NewRequest(http.MethodGet, "/stocks", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var bank model.BankState
	json.NewDecoder(w.Body).Decode(&bank)

	if len(bank.Stocks) != 1 || bank.Stocks[0].Quantity != 3 {
		t.Fatalf("expected bank AAPL qty 3 after buy+sell, got %+v", bank.Stocks)
	}
}
