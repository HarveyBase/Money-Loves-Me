package strategy

import (
	"time"

	"github.com/shopspring/decimal"

	"money-loves-me/pkg/binance"
)

// StrategyParams 是参数名称到其 decimal 值的映射。
type StrategyParams map[string]decimal.Decimal

// SignalDirection 表示交易信号的方向。
type SignalDirection string

const (
	SignalBuy  SignalDirection = "BUY"
	SignalSell SignalDirection = "SELL"
)

// SignalReason 记录策略生成信号时的决策上下文。
type SignalReason struct {
	Indicators  map[string]float64 `json:"indicators"`
	TriggerRule string             `json:"trigger_rule"`
	MarketState string             `json:"market_state"`
}

// Signal 表示策略生成的交易信号。
type Signal struct {
	Strategy       string          `json:"strategy"`
	Symbol         string          `json:"symbol"`
	Side           SignalDirection `json:"side"`
	Price          decimal.Decimal `json:"price"`
	Quantity       decimal.Decimal `json:"quantity"`
	ExpectedProfit decimal.Decimal `json:"expected_profit"` // 扣除手续费前的预期利润
	Reason         SignalReason    `json:"reason"`
	Timestamp      time.Time       `json:"timestamp"`
}

// FeeRate 保存 maker 和 taker 手续费率。
type FeeRate struct {
	Maker decimal.Decimal
	Taker decimal.Decimal
}

// Strategy 定义所有交易策略必须实现的接口。
type Strategy interface {
	// Name 返回策略名称。
	Name() string

	// Calculate 分析 K 线数据并返回交易信号，如果没有信号则返回 nil。
	Calculate(klines []binance.Kline) *Signal

	// GetParams 返回当前策略参数。
	GetParams() StrategyParams

	// SetParams 更新策略参数。如果参数无效则返回错误。
	SetParams(params StrategyParams) error

	// EstimateFee 根据给定的价格、数量和费率估算交易手续费。
	EstimateFee(price, quantity, feeRate decimal.Decimal) decimal.Decimal
}
