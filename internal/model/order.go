package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Order 表示 orders 数据表。
type Order struct {
	ID             int64           `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Symbol         string          `gorm:"column:symbol;type:varchar(20);not null;index:idx_orders_symbol" json:"symbol"`
	Side           string          `gorm:"column:side;type:varchar(4);not null" json:"side"`
	Type           string          `gorm:"column:type;type:varchar(30);not null" json:"type"`
	Quantity       decimal.Decimal `gorm:"column:quantity;type:decimal(20,8);not null" json:"quantity"`
	Price          decimal.Decimal `gorm:"column:price;type:decimal(20,8);not null;default:0" json:"price"`
	StopPrice      decimal.Decimal `gorm:"column:stop_price;type:decimal(20,8);not null;default:0" json:"stop_price"`
	Status         string          `gorm:"column:status;type:varchar(20);not null;default:SUBMITTED;index:idx_orders_status" json:"status"`
	BinanceOrderID *int64          `gorm:"column:binance_order_id;index:idx_orders_binance_order_id" json:"binance_order_id"`
	Fee            decimal.Decimal `gorm:"column:fee;type:decimal(20,8);not null;default:0" json:"fee"`
	FeeAsset       string          `gorm:"column:fee_asset;type:varchar(10);not null;default:''" json:"fee_asset"`
	StrategyName   string          `gorm:"column:strategy_name;type:varchar(50);not null;default:'';index:idx_orders_strategy_name" json:"strategy_name"`
	CreatedAt      time.Time       `gorm:"column:created_at;autoCreateTime;index:idx_orders_created_at" json:"created_at"`
	UpdatedAt      time.Time       `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`

	// 一对多（Has-many）关联关系
	Trades []Trade `gorm:"foreignKey:OrderID;references:ID" json:"trades,omitempty"`
}

// TableName 覆盖默认的表名。
func (Order) TableName() string {
	return "orders"
}
