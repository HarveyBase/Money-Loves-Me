package model

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// AccountSnapshot represents the account_snapshots table.
type AccountSnapshot struct {
	ID             int64           `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TotalValueUSDT decimal.Decimal `gorm:"column:total_value_usdt;type:decimal(20,8);not null" json:"total_value_usdt"`
	Balances       json.RawMessage `gorm:"column:balances;type:json;not null" json:"balances"`
	SnapshotAt     time.Time       `gorm:"column:snapshot_at;not null;index:idx_account_snapshots_snapshot_at" json:"snapshot_at"`
}

// TableName overrides the default table name.
func (AccountSnapshot) TableName() string {
	return "account_snapshots"
}
