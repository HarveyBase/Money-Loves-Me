package binance

import (
	"time"

	"github.com/shopspring/decimal"
)

// Kline 表示单个蜡烛图（OHLCV）数据点。
type Kline struct {
	OpenTime  time.Time
	Open      decimal.Decimal
	High      decimal.Decimal
	Low       decimal.Decimal
	Close     decimal.Decimal
	Volume    decimal.Decimal
	CloseTime time.Time
}

// CreateOrderRequest 包含创建新订单所需的参数。
type CreateOrderRequest struct {
	Symbol      string
	Side        string // BUY / SELL
	Type        string // LIMIT / MARKET / STOP_LOSS_LIMIT / TAKE_PROFIT_LIMIT
	Quantity    decimal.Decimal
	Price       decimal.Decimal
	StopPrice   decimal.Decimal // 止损/止盈的触发价格
	TimeInForce string          // GTC, IOC, FOK
}

// OrderResponse 是 Binance 在订单操作后返回的响应。
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

// OrderFill 表示订单响应中的单笔成交记录。
type OrderFill struct {
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
}

// AccountInfo 包含账户余额和权限信息。
type AccountInfo struct {
	MakerCommission int       `json:"makerCommission"`
	TakerCommission int       `json:"takerCommission"`
	CanTrade        bool      `json:"canTrade"`
	CanWithdraw     bool      `json:"canWithdraw"`
	CanDeposit      bool      `json:"canDeposit"`
	UpdateTime      int64     `json:"updateTime"`
	Balances        []Balance `json:"balances"`
}

// Balance 表示单个资产的余额。
type Balance struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

// ExchangeInfo 包含交易规则和交易对信息。
type ExchangeInfo struct {
	Timezone   string       `json:"timezone"`
	ServerTime int64        `json:"serverTime"`
	Symbols    []SymbolInfo `json:"symbols"`
}

// SymbolInfo 描述单个交易对的规则。
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

// SymbolFilter 表示交易对的交易规则过滤器。
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

// OrderBook 表示某个交易对的当前订单簿。
type OrderBook struct {
	Symbol     string
	Bids       []PriceLevel // 买方，价格降序
	Asks       []PriceLevel // 卖方，价格升序
	UpdateTime time.Time
}

// PriceLevel 是订单簿中的单个价格/数量条目。
type PriceLevel struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
}

// binanceKlineRaw 是 Binance K 线接口返回的原始 JSON 数组。
// 每个元素为：[openTime, open, high, low, close, volume, closeTime, ...]
type binanceKlineRaw = []interface{}

// binanceOrderBookRaw 是 Binance 订单簿接口返回的原始 JSON 响应。
type binanceOrderBookRaw struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}
