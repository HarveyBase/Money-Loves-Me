package model

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// AccountSnapshot 表示 account_snapshots 数据表。
type AccountSnapshot struct {
	ID             int64           `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TotalValueUSDT decimal.Decimal `gorm:"column:total_value_usdt;type:decimal(20,8);not null" json:"total_value_usdt"`
	Balances       json.RawMessage `gorm:"column:balances;type:json;not null" json:"balances"`
	SnapshotAt     time.Time       `gorm:"column:snapshot_at;not null;index:idx_account_snapshots_snapshot_at" json:"snapshot_at"`
}

// TableName 覆盖默认的表名。
func (AccountSnapshot) TableName() string {
	return "account_snapshots"
}
