package model

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// Trade represents the trades table.
type Trade struct {
	ID             int64           `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderID        int64           `gorm:"column:order_id;not null;index:idx_trades_order_id" json:"order_id"`
	Symbol         string          `gorm:"column:symbol;type:varchar(20);not null;index:idx_trades_symbol" json:"symbol"`
	Side           string          `gorm:"column:side;type:varchar(4);not null" json:"side"`
	Price          decimal.Decimal `gorm:"column:price;type:decimal(20,8);not null" json:"price"`
	Quantity       decimal.Decimal `gorm:"column:quantity;type:decimal(20,8);not null" json:"quantity"`
	Amount         decimal.Decimal `gorm:"column:amount;type:decimal(20,8);not null" json:"amount"`
	Fee            decimal.Decimal `gorm:"column:fee;type:decimal(20,8);not null;default:0" json:"fee"`
	FeeAsset       string          `gorm:"column:fee_asset;type:varchar(10);not null;default:''" json:"fee_asset"`
	StrategyName   string          `gorm:"column:strategy_name;type:varchar(50);not null;default:'';index:idx_trades_strategy_name" json:"strategy_name"`
	DecisionReason json.RawMessage `gorm:"column:decision_reason;type:json" json:"decision_reason"`
	BalanceBefore  decimal.Decimal `gorm:"column:balance_before;type:decimal(20,8);not null;default:0" json:"balance_before"`
	BalanceAfter   decimal.Decimal `gorm:"column:balance_after;type:decimal(20,8);not null;default:0" json:"balance_after"`
	ExecutedAt     time.Time       `gorm:"column:executed_at;not null;index:idx_trades_executed_at" json:"executed_at"`

	// Belongs-to relationship
	Order Order `gorm:"foreignKey:OrderID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE" json:"order,omitempty"`
}

// TableName overrides the default table name.
func (Trade) TableName() string {
	return "trades"
}

// DecisionReasonJSON is the structured representation of the decision_reason JSON column.
type DecisionReasonJSON struct {
	Indicators  map[string]float64 `json:"indicators"`
	TriggerRule string             `json:"trigger_rule"`
	MarketState string             `json:"market_state"`
}
