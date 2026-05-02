package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/szymoniwaniuk/stock-market-go/internal/model"
)

const (
	bankStocksKey = "bank:stocks"
	auditLogKey   = "audit_log"
)

func walletKey(walletID string) string {
	return fmt.Sprintf("wallet:%s:stocks", walletID)
}

// Lua script for atomic buy: decrement bank, increment wallet, append audit log.
// KEYS[1] = bank:stocks, KEYS[2] = wallet:<id>:stocks, KEYS[3] = audit_log
// ARGV[1] = stock_name, ARGV[2] = wallet_id, ARGV[3] = log entry JSON
var buyScript = redis.NewScript(`
local bank_qty = tonumber(redis.call('HGET', KEYS[1], ARGV[1])) or -1
if bank_qty == -1 then
	return {err = "stock_not_found"}
end
if bank_qty <= 0 then
	return {err = "insufficient_bank"}
end
redis.call('HINCRBY', KEYS[1], ARGV[1], -1)
redis.call('HINCRBY', KEYS[2], ARGV[1], 1)
redis.call('RPUSH', KEYS[3], ARGV[3])
return 1
`)

// Lua script for atomic sell: decrement wallet, increment bank, append audit log.
// KEYS[1] = bank:stocks, KEYS[2] = wallet:<id>:stocks, KEYS[3] = audit_log
// ARGV[1] = stock_name, ARGV[2] = wallet_id, ARGV[3] = log entry JSON
var sellScript = redis.NewScript(`
local exists = redis.call('HEXISTS', KEYS[1], ARGV[1])
if exists == 0 then
	return {err = "stock_not_found"}
end
local wallet_qty = tonumber(redis.call('HGET', KEYS[2], ARGV[1])) or 0
if wallet_qty <= 0 then
	return {err = "insufficient_wallet"}
end
redis.call('HINCRBY', KEYS[2], ARGV[1], -1)
redis.call('HINCRBY', KEYS[1], ARGV[1], 1)
redis.call('RPUSH', KEYS[3], ARGV[3])
return 1
`)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(addr string) *RedisStore {
	return &RedisStore{
		client: redis.NewClient(&redis.Options{
			Addr: addr,
		}),
	}
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *RedisStore) SetBankStocks(ctx context.Context, stocks []model.Stock) error {
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, bankStocksKey)

	if len(stocks) > 0 {
		fields := make(map[string]interface{}, len(stocks))
		for _, st := range stocks {
			fields[st.Name] = st.Quantity
		}
		pipe.HSet(ctx, bankStocksKey, fields)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) GetBankStocks(ctx context.Context) ([]model.Stock, error) {
	result, err := s.client.HGetAll(ctx, bankStocksKey).Result()
	if err != nil {
		return nil, err
	}

	stocks := make([]model.Stock, 0, len(result))
	for name, qtyStr := range result {
		qty, _ := strconv.Atoi(qtyStr)
		stocks = append(stocks, model.Stock{Name: name, Quantity: qty})
	}
	return stocks, nil
}

func (s *RedisStore) GetWallet(ctx context.Context, walletID string) (*model.WalletResponse, error) {
	result, err := s.client.HGetAll(ctx, walletKey(walletID)).Result()
	if err != nil {
		return nil, err
	}

	stocks := make([]model.Stock, 0, len(result))
	for name, qtyStr := range result {
		qty, _ := strconv.Atoi(qtyStr)
		if qty > 0 {
			stocks = append(stocks, model.Stock{Name: name, Quantity: qty})
		}
	}

	return &model.WalletResponse{
		ID:     walletID,
		Stocks: stocks,
	}, nil
}

func (s *RedisStore) GetWalletStock(ctx context.Context, walletID, stockName string) (int, error) {
	val, err := s.client.HGet(ctx, walletKey(walletID), stockName).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	qty, _ := strconv.Atoi(val)
	return qty, nil
}

func (s *RedisStore) ExecuteTrade(ctx context.Context, walletID, stockName, tradeType string) error {
	logEntry, err := json.Marshal(model.LogEntry{
		Type:      tradeType,
		WalletID:  walletID,
		StockName: stockName,
	})
	if err != nil {
		return err
	}

	keys := []string{bankStocksKey, walletKey(walletID), auditLogKey}
	args := []interface{}{stockName, walletID, string(logEntry)}

	var script *redis.Script
	switch tradeType {
	case "buy":
		script = buyScript
	case "sell":
		script = sellScript
	default:
		return ErrInvalidTradeType
	}

	_, err = script.Run(ctx, s.client, keys, args...).Result()
	if err != nil {
		return mapScriptError(err)
	}
	return nil
}

func (s *RedisStore) GetLog(ctx context.Context) ([]model.LogEntry, error) {
	vals, err := s.client.LRange(ctx, auditLogKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]model.LogEntry, 0, len(vals))
	for _, v := range vals {
		var entry model.LogEntry
		if err := json.Unmarshal([]byte(v), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func mapScriptError(err error) error {
	switch err.Error() {
	case "stock_not_found":
		return ErrStockNotFound
	case "insufficient_bank":
		return ErrInsufficientBank
	case "insufficient_wallet":
		return ErrInsufficientWallet
	default:
		return err
	}
}
