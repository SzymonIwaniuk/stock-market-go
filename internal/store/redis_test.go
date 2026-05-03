package store

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/szymoniwaniuk/stock-market-go/internal/model"
)

func setupStore(t *testing.T) *RedisStore {
	t.Helper()
	mr := miniredis.RunT(t)
	return NewRedisStore(mr.Addr())
}

func TestSetAndGetBankStocks(t *testing.T) {
	tests := []struct {
		name     string
		stocks   []model.Stock
		expected int
	}{
		{
			name:     "multiple stocks",
			stocks:   []model.Stock{{Name: "AAPL", Quantity: 10}, {Name: "GOOG", Quantity: 5}},
			expected: 2,
		},
		{
			name:     "single stock",
			stocks:   []model.Stock{{Name: "AAPL", Quantity: 100}},
			expected: 1,
		},
		{
			name:     "empty stocks",
			stocks:   []model.Stock{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupStore(t)
			ctx := context.Background()

			err := s.SetBankStocks(ctx, tt.stocks)
			require.NoError(t, err)

			stocks, err := s.GetBankStocks(ctx)
			require.NoError(t, err)
			assert.Len(t, stocks, tt.expected)
		})
	}
}

func TestSetBankStocksOverwrites(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	require.NoError(t, s.SetBankStocks(ctx, []model.Stock{{Name: "AAPL", Quantity: 10}}))
	require.NoError(t, s.SetBankStocks(ctx, []model.Stock{{Name: "GOOG", Quantity: 5}}))

	stocks, err := s.GetBankStocks(ctx)
	require.NoError(t, err)
	require.Len(t, stocks, 1)
	assert.Equal(t, "GOOG", stocks[0].Name)
	assert.Equal(t, 5, stocks[0].Quantity)
}

func TestExecuteTrade(t *testing.T) {
	tests := []struct {
		name        string
		bankStocks  []model.Stock
		tradeType   string
		stockName   string
		expectedErr error
	}{
		{
			name:        "buy success",
			bankStocks:  []model.Stock{{Name: "AAPL", Quantity: 5}},
			tradeType:   "buy",
			stockName:   "AAPL",
			expectedErr: nil,
		},
		{
			name:        "buy stock not found",
			bankStocks:  []model.Stock{},
			tradeType:   "buy",
			stockName:   "NOPE",
			expectedErr: ErrStockNotFound,
		},
		{
			name:        "buy insufficient bank",
			bankStocks:  []model.Stock{{Name: "AAPL", Quantity: 0}},
			tradeType:   "buy",
			stockName:   "AAPL",
			expectedErr: ErrInsufficientBank,
		},
		{
			name:        "sell insufficient wallet",
			bankStocks:  []model.Stock{{Name: "AAPL", Quantity: 5}},
			tradeType:   "sell",
			stockName:   "AAPL",
			expectedErr: ErrInsufficientWallet,
		},
		{
			name:        "sell stock not found",
			bankStocks:  []model.Stock{},
			tradeType:   "sell",
			stockName:   "NOPE",
			expectedErr: ErrStockNotFound,
		},
		{
			name:        "invalid trade type",
			bankStocks:  []model.Stock{{Name: "AAPL", Quantity: 5}},
			tradeType:   "hold",
			stockName:   "AAPL",
			expectedErr: ErrInvalidTradeType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupStore(t)
			ctx := context.Background()

			if len(tt.bankStocks) > 0 {
				require.NoError(t, s.SetBankStocks(ctx, tt.bankStocks))
			}

			err := s.ExecuteTrade(ctx, "w1", tt.stockName, tt.tradeType)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuyDecrementsBankIncrementsWallet(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	require.NoError(t, s.SetBankStocks(ctx, []model.Stock{{Name: "AAPL", Quantity: 3}}))
	require.NoError(t, s.ExecuteTrade(ctx, "w1", "AAPL", "buy"))

	stocks, err := s.GetBankStocks(ctx)
	require.NoError(t, err)
	require.Len(t, stocks, 1)
	assert.Equal(t, 2, stocks[0].Quantity)

	qty, err := s.GetWalletStock(ctx, "w1", "AAPL")
	require.NoError(t, err)
	assert.Equal(t, 1, qty)
}

func TestSellIncrementsBank(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	require.NoError(t, s.SetBankStocks(ctx, []model.Stock{{Name: "AAPL", Quantity: 3}}))
	require.NoError(t, s.ExecuteTrade(ctx, "w1", "AAPL", "buy"))
	require.NoError(t, s.ExecuteTrade(ctx, "w1", "AAPL", "sell"))

	stocks, err := s.GetBankStocks(ctx)
	require.NoError(t, err)
	require.Len(t, stocks, 1)
	assert.Equal(t, 3, stocks[0].Quantity)

	qty, err := s.GetWalletStock(ctx, "w1", "AAPL")
	require.NoError(t, err)
	assert.Equal(t, 0, qty)
}

func TestGetWallet(t *testing.T) {
	tests := []struct {
		name        string
		buyCount    int
		expectedLen int
		expectedQty int
	}{
		{
			name:        "wallet with stocks",
			buyCount:    3,
			expectedLen: 1,
			expectedQty: 3,
		},
		{
			name:        "empty wallet",
			buyCount:    0,
			expectedLen: 0,
			expectedQty: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := setupStore(t)
			ctx := context.Background()

			require.NoError(t, s.SetBankStocks(ctx, []model.Stock{{Name: "AAPL", Quantity: 10}}))
			for i := 0; i < tt.buyCount; i++ {
				require.NoError(t, s.ExecuteTrade(ctx, "w1", "AAPL", "buy"))
			}

			wallet, err := s.GetWallet(ctx, "w1")
			require.NoError(t, err)
			assert.Equal(t, "w1", wallet.ID)
			assert.Len(t, wallet.Stocks, tt.expectedLen)

			if tt.expectedLen > 0 {
				assert.Equal(t, tt.expectedQty, wallet.Stocks[0].Quantity)
			}
		})
	}
}

func TestGetWalletStockNonExistent(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	qty, err := s.GetWalletStock(ctx, "w1", "AAPL")
	require.NoError(t, err)
	assert.Equal(t, 0, qty)
}

func TestGetLog(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	require.NoError(t, s.SetBankStocks(ctx, []model.Stock{{Name: "AAPL", Quantity: 10}}))
	require.NoError(t, s.ExecuteTrade(ctx, "w1", "AAPL", "buy"))
	require.NoError(t, s.ExecuteTrade(ctx, "w1", "AAPL", "sell"))
	require.NoError(t, s.ExecuteTrade(ctx, "w2", "AAPL", "buy"))

	entries, err := s.GetLog(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	assert.Equal(t, "buy", entries[0].Type)
	assert.Equal(t, "w1", entries[0].WalletID)
	assert.Equal(t, "sell", entries[1].Type)
	assert.Equal(t, "buy", entries[2].Type)
	assert.Equal(t, "w2", entries[2].WalletID)
}

func TestGetLogEmpty(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	entries, err := s.GetLog(ctx)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestFailedTradesNotLogged(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	_ = s.ExecuteTrade(ctx, "w1", "NOPE", "buy")

	require.NoError(t, s.SetBankStocks(ctx, []model.Stock{{Name: "AAPL", Quantity: 0}}))
	_ = s.ExecuteTrade(ctx, "w1", "AAPL", "buy")
	_ = s.ExecuteTrade(ctx, "w1", "AAPL", "sell")

	entries, err := s.GetLog(ctx)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestFlush(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	require.NoError(t, s.SetBankStocks(ctx, []model.Stock{{Name: "AAPL", Quantity: 10}}))
	require.NoError(t, s.ExecuteTrade(ctx, "w1", "AAPL", "buy"))

	require.NoError(t, s.Flush(ctx))

	stocks, err := s.GetBankStocks(ctx)
	require.NoError(t, err)
	assert.Empty(t, stocks)

	entries, err := s.GetLog(ctx)
	require.NoError(t, err)
	assert.Empty(t, entries)

	wallet, err := s.GetWallet(ctx, "w1")
	require.NoError(t, err)
	assert.Empty(t, wallet.Stocks)
}

func TestPing(t *testing.T) {
	s := setupStore(t)
	assert.NoError(t, s.Ping(context.Background()))
}
