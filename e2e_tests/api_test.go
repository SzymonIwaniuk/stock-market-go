//go:build e2e

package e2e_tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/szymoniwaniuk/stock-market-go/internal/model"
)

type E2ETestSuite struct {
	suite.Suite
	serverURL string
	client    *http.Client
}

func (s *E2ETestSuite) SetupSuite() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	s.serverURL = fmt.Sprintf("http://localhost:%s", port)
	s.client = &http.Client{Timeout: 5 * time.Second}

	// Wait for the service to be ready
	for i := 0; i < 30; i++ {
		resp, err := s.client.Get(s.serverURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	s.T().Fatal("service did not become ready in time")
}

func (s *E2ETestSuite) SetupTest() {
	resp, err := s.client.Post(s.serverURL+"/reset", "application/json", nil)
	require.NoError(s.T(), err)
	resp.Body.Close()
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
}

func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(E2ETestSuite))
}

func (s *E2ETestSuite) postJSON(path string, payload interface{}) *http.Response {
	body, err := json.Marshal(payload)
	require.NoError(s.T(), err)
	resp, err := s.client.Post(s.serverURL+path, "application/json", bytes.NewBuffer(body))
	require.NoError(s.T(), err)
	return resp
}

func (s *E2ETestSuite) get(path string) *http.Response {
	resp, err := s.client.Get(s.serverURL + path)
	require.NoError(s.T(), err)
	return resp
}

func (s *E2ETestSuite) readBody(resp *http.Response) string {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(s.T(), err)
	return string(b)
}

// --- Bank Tests ---

func (s *E2ETestSuite) TestSetAndGetBankStocks() {
	stocks := model.BankState{Stocks: []model.Stock{
		{Name: "AAPL", Quantity: 100},
		{Name: "GOOG", Quantity: 50},
	}}

	resp := s.postJSON("/stocks", stocks)
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)

	resp = s.get("/stocks")
	var bank model.BankState
	json.NewDecoder(resp.Body).Decode(&bank)
	resp.Body.Close()

	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	assert.Len(s.T(), bank.Stocks, 2)

	stockMap := make(map[string]int)
	for _, st := range bank.Stocks {
		stockMap[st.Name] = st.Quantity
	}
	assert.Equal(s.T(), 100, stockMap["AAPL"])
	assert.Equal(s.T(), 50, stockMap["GOOG"])
}

func (s *E2ETestSuite) TestSetBankOverwritesPreviousState() {
	resp := s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 10}}})
	resp.Body.Close()

	resp = s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "GOOG", Quantity: 5}}})
	resp.Body.Close()

	resp = s.get("/stocks")
	var bank model.BankState
	json.NewDecoder(resp.Body).Decode(&bank)
	resp.Body.Close()

	assert.Len(s.T(), bank.Stocks, 1)
	assert.Equal(s.T(), "GOOG", bank.Stocks[0].Name)
	assert.Equal(s.T(), 5, bank.Stocks[0].Quantity)
}

func (s *E2ETestSuite) TestEmptyBankInitially() {
	resp := s.get("/stocks")
	var bank model.BankState
	json.NewDecoder(resp.Body).Decode(&bank)
	resp.Body.Close()

	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	assert.Empty(s.T(), bank.Stocks)
}

// --- Buy Tests ---

func (s *E2ETestSuite) TestBuySuccess() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 10}}}).Body.Close()

	resp := s.postJSON("/wallets/wallet1/stocks/AAPL", model.TradeRequest{Type: "buy"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)

	// Wallet should have 1
	resp = s.get("/wallets/wallet1/stocks/AAPL")
	body := s.readBody(resp)
	assert.Equal(s.T(), "1", body)

	// Bank should have 9
	resp = s.get("/stocks")
	var bank model.BankState
	json.NewDecoder(resp.Body).Decode(&bank)
	resp.Body.Close()
	assert.Equal(s.T(), 9, bank.Stocks[0].Quantity)
}

func (s *E2ETestSuite) TestBuyStockNotFound() {
	resp := s.postJSON("/wallets/wallet1/stocks/NONEXISTENT", model.TradeRequest{Type: "buy"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusNotFound, resp.StatusCode)
}

func (s *E2ETestSuite) TestBuyInsufficientBankStock() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 0}}}).Body.Close()

	resp := s.postJSON("/wallets/wallet1/stocks/AAPL", model.TradeRequest{Type: "buy"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
}

func (s *E2ETestSuite) TestBuyCreatesWalletAutomatically() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 5}}}).Body.Close()

	resp := s.postJSON("/wallets/new_wallet/stocks/AAPL", model.TradeRequest{Type: "buy"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)

	resp = s.get("/wallets/new_wallet")
	var wallet model.WalletResponse
	json.NewDecoder(resp.Body).Decode(&wallet)
	resp.Body.Close()

	assert.Equal(s.T(), "new_wallet", wallet.ID)
	assert.Len(s.T(), wallet.Stocks, 1)
	assert.Equal(s.T(), "AAPL", wallet.Stocks[0].Name)
	assert.Equal(s.T(), 1, wallet.Stocks[0].Quantity)
}

func (s *E2ETestSuite) TestBuyMultipleStocks() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{
		{Name: "AAPL", Quantity: 10},
		{Name: "GOOG", Quantity: 5},
	}}).Body.Close()

	s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "buy"}).Body.Close()
	s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "buy"}).Body.Close()
	s.postJSON("/wallets/w1/stocks/GOOG", model.TradeRequest{Type: "buy"}).Body.Close()

	resp := s.get("/wallets/w1")
	var wallet model.WalletResponse
	json.NewDecoder(resp.Body).Decode(&wallet)
	resp.Body.Close()

	stockMap := make(map[string]int)
	for _, st := range wallet.Stocks {
		stockMap[st.Name] = st.Quantity
	}
	assert.Equal(s.T(), 2, stockMap["AAPL"])
	assert.Equal(s.T(), 1, stockMap["GOOG"])
}

// --- Sell Tests ---

func (s *E2ETestSuite) TestSellSuccess() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 10}}}).Body.Close()
	s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "buy"}).Body.Close()

	resp := s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "sell"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)

	// Wallet should have 0
	resp = s.get("/wallets/w1/stocks/AAPL")
	body := s.readBody(resp)
	assert.Equal(s.T(), "0", body)

	// Bank should be back to 10
	resp = s.get("/stocks")
	var bank model.BankState
	json.NewDecoder(resp.Body).Decode(&bank)
	resp.Body.Close()
	assert.Equal(s.T(), 10, bank.Stocks[0].Quantity)
}

func (s *E2ETestSuite) TestSellInsufficientWalletStock() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 10}}}).Body.Close()

	resp := s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "sell"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
}

func (s *E2ETestSuite) TestSellStockNotFound() {
	resp := s.postJSON("/wallets/w1/stocks/NONEXISTENT", model.TradeRequest{Type: "sell"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusNotFound, resp.StatusCode)
}

// --- Wallet Tests ---

func (s *E2ETestSuite) TestGetNonExistentWallet() {
	resp := s.get("/wallets/does_not_exist")
	var wallet model.WalletResponse
	json.NewDecoder(resp.Body).Decode(&wallet)
	resp.Body.Close()

	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	assert.Equal(s.T(), "does_not_exist", wallet.ID)
	assert.Empty(s.T(), wallet.Stocks)
}

func (s *E2ETestSuite) TestGetWalletStockQuantity() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 10}}}).Body.Close()

	for i := 0; i < 3; i++ {
		s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "buy"}).Body.Close()
	}

	resp := s.get("/wallets/w1/stocks/AAPL")
	body := s.readBody(resp)
	assert.Equal(s.T(), "3", body)
}

func (s *E2ETestSuite) TestGetWalletStockZeroForUnowned() {
	resp := s.get("/wallets/w1/stocks/AAPL")
	body := s.readBody(resp)
	assert.Equal(s.T(), "0", body)
}

// --- Audit Log Tests ---

func (s *E2ETestSuite) TestAuditLogRecordsOperations() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 10}}}).Body.Close()

	s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "buy"}).Body.Close()
	s.postJSON("/wallets/w2/stocks/AAPL", model.TradeRequest{Type: "buy"}).Body.Close()
	s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "sell"}).Body.Close()

	resp := s.get("/log")
	var log model.LogResponse
	json.NewDecoder(resp.Body).Decode(&log)
	resp.Body.Close()

	assert.Len(s.T(), log.Log, 3)

	assert.Equal(s.T(), "buy", log.Log[0].Type)
	assert.Equal(s.T(), "w1", log.Log[0].WalletID)
	assert.Equal(s.T(), "AAPL", log.Log[0].StockName)

	assert.Equal(s.T(), "buy", log.Log[1].Type)
	assert.Equal(s.T(), "w2", log.Log[1].WalletID)

	assert.Equal(s.T(), "sell", log.Log[2].Type)
	assert.Equal(s.T(), "w1", log.Log[2].WalletID)
}

func (s *E2ETestSuite) TestAuditLogDoesNotRecordFailedOperations() {
	// These should all fail
	s.postJSON("/wallets/w1/stocks/NOPE", model.TradeRequest{Type: "buy"}).Body.Close()

	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 0}}}).Body.Close()
	s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "buy"}).Body.Close()
	s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "sell"}).Body.Close()

	resp := s.get("/log")
	var log model.LogResponse
	json.NewDecoder(resp.Body).Decode(&log)
	resp.Body.Close()

	assert.Empty(s.T(), log.Log)
}

func (s *E2ETestSuite) TestAuditLogOrderPreserved() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 10}}}).Body.Close()

	for i := 0; i < 5; i++ {
		walletID := fmt.Sprintf("w%d", i)
		s.postJSON(fmt.Sprintf("/wallets/%s/stocks/AAPL", walletID), model.TradeRequest{Type: "buy"}).Body.Close()
	}

	resp := s.get("/log")
	var log model.LogResponse
	json.NewDecoder(resp.Body).Decode(&log)
	resp.Body.Close()

	require.Len(s.T(), log.Log, 5)
	for i := 0; i < 5; i++ {
		assert.Equal(s.T(), fmt.Sprintf("w%d", i), log.Log[i].WalletID)
	}
}

// --- Invalid Request Tests ---

func (s *E2ETestSuite) TestInvalidTradeType() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 10}}}).Body.Close()

	resp := s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "hold"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
}

func (s *E2ETestSuite) TestInvalidJSONBody() {
	resp, err := s.client.Post(
		s.serverURL+"/wallets/w1/stocks/AAPL",
		"application/json",
		bytes.NewBufferString(`{invalid json`),
	)
	require.NoError(s.T(), err)
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
}

// --- Full Flow Test ---

func (s *E2ETestSuite) TestFullAPIFlow() {
	// 1. Bank starts empty
	resp := s.get("/stocks")
	var bank model.BankState
	json.NewDecoder(resp.Body).Decode(&bank)
	resp.Body.Close()
	assert.Empty(s.T(), bank.Stocks)

	// 2. Set bank stocks
	resp = s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{
		{Name: "AAPL", Quantity: 5},
		{Name: "GOOG", Quantity: 3},
	}})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)

	// 3. Buy AAPL for wallet1
	resp = s.postJSON("/wallets/wallet1/stocks/AAPL", model.TradeRequest{Type: "buy"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)

	// 4. Buy GOOG for wallet1
	resp = s.postJSON("/wallets/wallet1/stocks/GOOG", model.TradeRequest{Type: "buy"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)

	// 5. Buy AAPL for wallet2
	resp = s.postJSON("/wallets/wallet2/stocks/AAPL", model.TradeRequest{Type: "buy"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)

	// 6. Verify wallet1
	resp = s.get("/wallets/wallet1")
	var w1 model.WalletResponse
	json.NewDecoder(resp.Body).Decode(&w1)
	resp.Body.Close()
	assert.Equal(s.T(), "wallet1", w1.ID)
	assert.Len(s.T(), w1.Stocks, 2)

	// 7. Verify bank decremented
	resp = s.get("/stocks")
	json.NewDecoder(resp.Body).Decode(&bank)
	resp.Body.Close()
	bankMap := make(map[string]int)
	for _, st := range bank.Stocks {
		bankMap[st.Name] = st.Quantity
	}
	assert.Equal(s.T(), 3, bankMap["AAPL"])
	assert.Equal(s.T(), 2, bankMap["GOOG"])

	// 8. Sell AAPL from wallet1 back
	resp = s.postJSON("/wallets/wallet1/stocks/AAPL", model.TradeRequest{Type: "sell"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)

	// 9. Verify wallet1 AAPL is 0
	resp = s.get("/wallets/wallet1/stocks/AAPL")
	body := s.readBody(resp)
	assert.Equal(s.T(), "0", body)

	// 10. Bank AAPL should be 4 now
	resp = s.get("/stocks")
	json.NewDecoder(resp.Body).Decode(&bank)
	resp.Body.Close()
	bankMap = make(map[string]int)
	for _, st := range bank.Stocks {
		bankMap[st.Name] = st.Quantity
	}
	assert.Equal(s.T(), 4, bankMap["AAPL"])

	// 11. Verify full audit log
	resp = s.get("/log")
	var log model.LogResponse
	json.NewDecoder(resp.Body).Decode(&log)
	resp.Body.Close()
	assert.Len(s.T(), log.Log, 4)
	assert.Equal(s.T(), "buy", log.Log[0].Type)
	assert.Equal(s.T(), "wallet1", log.Log[0].WalletID)
	assert.Equal(s.T(), "AAPL", log.Log[0].StockName)
	assert.Equal(s.T(), "sell", log.Log[3].Type)

	// 12. Try to sell again (should fail, wallet is empty for AAPL)
	resp = s.postJSON("/wallets/wallet1/stocks/AAPL", model.TradeRequest{Type: "sell"})
	resp.Body.Close()
	assert.Equal(s.T(), http.StatusBadRequest, resp.StatusCode)
}

// --- Chaos / HA Test ---

func (s *E2ETestSuite) TestChaosServiceSurvives() {
	s.postJSON("/stocks", model.BankState{Stocks: []model.Stock{{Name: "AAPL", Quantity: 10}}}).Body.Close()
	s.postJSON("/wallets/w1/stocks/AAPL", model.TradeRequest{Type: "buy"}).Body.Close()

	// Kill one instance
	resp, err := s.client.Post(s.serverURL+"/chaos", "application/json", nil)
	if err == nil {
		resp.Body.Close()
	}

	// Give nginx time to detect the failure and route elsewhere
	time.Sleep(3 * time.Second)

	// Service should still be available
	resp = s.get("/wallets/w1/stocks/AAPL")
	body := s.readBody(resp)
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	assert.Equal(s.T(), "1", body)
}
