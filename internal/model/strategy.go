package model

import (
	"encoding/json"
	"time"
)

// Strategy 表示 strategies 数据表。
type Strategy struct {
	ID        int             `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name      string          `gorm:"column:name;type:varchar(50);not null;uniqueIndex:uk_strategies_name" json:"name"`
	Type      string          `gorm:"column:type;type:varchar(50);not null" json:"type"`
	Params    json.RawMessage `gorm:"column:params;type:json;not null" json:"params"`
	Active    bool            `gorm:"column:active;not null;default:false" json:"active"`
	UpdatedAt time.Time       `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName 覆盖默认的表名。
func (Strategy) TableName() string {
	return "strategies"
}
