package model

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// BacktestResult 表示 backtest_results 数据表。
type BacktestResult struct {
	ID             int64           `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	StrategyName   string          `gorm:"column:strategy_name;type:varchar(50);not null;index:idx_backtest_results_strategy_name" json:"strategy_name"`
	Symbol         string          `gorm:"column:symbol;type:varchar(20);not null;index:idx_backtest_results_symbol" json:"symbol"`
	Params         json.RawMessage `gorm:"column:params;type:json;not null" json:"params"`
	StartTime      time.Time       `gorm:"column:start_time;not null" json:"start_time"`
	EndTime        time.Time       `gorm:"column:end_time;not null" json:"end_time"`
	InitialCapital decimal.Decimal `gorm:"column:initial_capital;type:decimal(20,8);not null" json:"initial_capital"`
	TotalReturn    decimal.Decimal `gorm:"column:total_return;type:decimal(20,8);not null;default:0" json:"total_return"`
	NetProfit      decimal.Decimal `gorm:"column:net_profit;type:decimal(20,8);not null;default:0" json:"net_profit"`
	MaxDrawdown    decimal.Decimal `gorm:"column:max_drawdown;type:decimal(20,8);not null;default:0" json:"max_drawdown"`
	WinRate        decimal.Decimal `gorm:"column:win_rate;type:decimal(20,8);not null;default:0" json:"win_rate"`
	ProfitFactor   decimal.Decimal `gorm:"column:profit_factor;type:decimal(20,8);not null;default:0" json:"profit_factor"`
	TotalTrades    int             `gorm:"column:total_trades;not null;default:0" json:"total_trades"`
	TotalFees      decimal.Decimal `gorm:"column:total_fees;type:decimal(20,8);not null;default:0" json:"total_fees"`
	EquityCurve    json.RawMessage `gorm:"column:equity_curve;type:json" json:"equity_curve"`
	Trades         json.RawMessage `gorm:"column:trades;type:json" json:"trades"`
	Slippage       decimal.Decimal `gorm:"column:slippage;type:decimal(20,8);not null;default:0" json:"slippage"`
	CreatedAt      time.Time       `gorm:"column:created_at;autoCreateTime;index:idx_backtest_results_created_at" json:"created_at"`
}

// TableName 覆盖默认的表名。
func (BacktestResult) TableName() string {
	return "backtest_results"
}
