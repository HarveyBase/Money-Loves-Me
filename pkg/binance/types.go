package binance

import (
	"time"

	"github.com/shopspring/decimal"
)

// Kline represents a single candlestick (OHLCV) data point.
type Kline struct {
	OpenTime  time.Time
	Open      decimal.Decimal
	High      decimal.Decimal
	Low       decimal.Decimal
	Close     decimal.Decimal
	Volume    decimal.Decimal
	CloseTime time.Time
}

// CreateOrderRequest holds the parameters for creating a new order.
type CreateOrderRequest struct {
	Symbol      string
	Side        string // BUY / SELL
	Type        string // LIMIT / MARKET / STOP_LOSS_LIMIT / TAKE_PROFIT_LIMIT
	Quantity    decimal.Decimal
	Price       decimal.Decimal
	StopPrice   decimal.Decimal // trigger price for stop-loss / take-profit
	TimeInForce string          // GTC, IOC, FOK
}

// OrderResponse is the response returned by Binance after order operations.
type OrderResponse struct {
	Symbol        string      `json:"symbol"`
	OrderID       int64       `json:"orderId"`
	ClientOrderID string      `json:"clientOrderId"`
	Price         string      `json:"price"`
	OrigQty       string      `json:"origQty"`
	ExecutedQty   string      `json:"executedQty"`
	Status        string      `json:"status"`
	Type          string      `json:"type"`
	Side          string      `json:"side"`
	TransactTime  int64       `json:"transactTime"`
	Fills         []OrderFill `json:"fills"`
}

// OrderFill represents a single fill within an order response.
type OrderFill struct {
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
}

// AccountInfo holds the account balance and permission information.
type AccountInfo struct {
	MakerCommission int       `json:"makerCommission"`
	TakerCommission int       `json:"takerCommission"`
	CanTrade        bool      `json:"canTrade"`
	CanWithdraw     bool      `json:"canWithdraw"`
	CanDeposit      bool      `json:"canDeposit"`
	UpdateTime      int64     `json:"updateTime"`
	Balances        []Balance `json:"balances"`
}

// Balance represents a single asset balance.
type Balance struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

// ExchangeInfo contains trading rules and symbol information.
type ExchangeInfo struct {
	Timezone   string       `json:"timezone"`
	ServerTime int64        `json:"serverTime"`
	Symbols    []SymbolInfo `json:"symbols"`
}

// SymbolInfo describes a single trading pair's rules.
type SymbolInfo struct {
	Symbol              string         `json:"symbol"`
	Status              string         `json:"status"`
	BaseAsset           string         `json:"baseAsset"`
	BaseAssetPrecision  int            `json:"baseAssetPrecision"`
	QuoteAsset          string         `json:"quoteAsset"`
	QuoteAssetPrecision int            `json:"quoteAssetPrecision"`
	OrderTypes          []string       `json:"orderTypes"`
	Filters             []SymbolFilter `json:"filters"`
}

// SymbolFilter represents a trading rule filter for a symbol.
type SymbolFilter struct {
	FilterType  string `json:"filterType"`
	MinPrice    string `json:"minPrice,omitempty"`
	MaxPrice    string `json:"maxPrice,omitempty"`
	TickSize    string `json:"tickSize,omitempty"`
	MinQty      string `json:"minQty,omitempty"`
	MaxQty      string `json:"maxQty,omitempty"`
	StepSize    string `json:"stepSize,omitempty"`
	MinNotional string `json:"minNotional,omitempty"`
}

// OrderBook represents the current order book for a symbol.
type OrderBook struct {
	Symbol     string
	Bids       []PriceLevel // buy side, price descending
	Asks       []PriceLevel // sell side, price ascending
	UpdateTime time.Time
}

// PriceLevel is a single price/quantity entry in the order book.
type PriceLevel struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
}

// binanceKlineRaw is the raw JSON array returned by the Binance klines endpoint.
// Each element is: [openTime, open, high, low, close, volume, closeTime, ...]
type binanceKlineRaw = []interface{}

// binanceOrderBookRaw is the raw JSON response from the Binance order book endpoint.
type binanceOrderBookRaw struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}
