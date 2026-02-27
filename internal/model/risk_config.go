package model

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// RiskConfig 表示 risk_configs 数据表。
type RiskConfig struct {
	ID                  int             `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	MaxOrderAmount      decimal.Decimal `gorm:"column:max_order_amount;type:decimal(20,8);not null" json:"max_order_amount"`
	MaxDailyLoss        decimal.Decimal `gorm:"column:max_daily_loss;type:decimal(20,8);not null" json:"max_daily_loss"`
	StopLossPercents    json.RawMessage `gorm:"column:stop_loss_percents;type:json;not null" json:"stop_loss_percents"`
	MaxPositionPercents json.RawMessage `gorm:"column:max_position_percents;type:json;not null" json:"max_position_percents"`
	UpdatedAt           time.Time       `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName 覆盖默认的表名。
func (RiskConfig) TableName() string {
	return "risk_configs"
}
