package store

import (
	"context"
	"errors"

	"github.com/szymoniwaniuk/stock-market-go/internal/model"
)

var (
	ErrStockNotFound      = errors.New("stock not found")
	ErrInsufficientBank   = errors.New("no stock available in bank")
	ErrInsufficientWallet = errors.New("no stock available in wallet")
	ErrInvalidTradeType   = errors.New("invalid trade type")
)

type Store interface {
	SetBankStocks(ctx context.Context, stocks []model.Stock) error
	GetBankStocks(ctx context.Context) ([]model.Stock, error)

	GetWallet(ctx context.Context, walletID string) (*model.WalletResponse, error)
	GetWalletStock(ctx context.Context, walletID, stockName string) (int, error)

	// ExecuteTrade atomically performs a buy or sell, updating bank, wallet, and audit log.
	ExecuteTrade(ctx context.Context, walletID, stockName, tradeType string) error

	GetLog(ctx context.Context) ([]model.LogEntry, error)
}
