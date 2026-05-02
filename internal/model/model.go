package model

type Stock struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type WalletResponse struct {
	ID     string  `json:"id"`
	Stocks []Stock `json:"stocks"`
}

type BankState struct {
	Stocks []Stock `json:"stocks"`
}

type LogEntry struct {
	Type      string `json:"type"`
	WalletID  string `json:"wallet_id"`
	StockName string `json:"stock_name"`
}

type LogResponse struct {
	Log []LogEntry `json:"log"`
}

type TradeRequest struct {
	Type string `json:"type"`
}
