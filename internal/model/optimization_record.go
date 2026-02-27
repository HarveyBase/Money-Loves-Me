package model

import (
	"encoding/json"
	"time"
)

// OptimizationRecord represents the optimization_records table.
type OptimizationRecord struct {
	ID            int64           `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	StrategyName  string          `gorm:"column:strategy_name;type:varchar(50);not null;index:idx_optimization_records_strategy_name" json:"strategy_name"`
	OldParams     json.RawMessage `gorm:"column:old_params;type:json;not null" json:"old_params"`
	NewParams     json.RawMessage `gorm:"column:new_params;type:json;not null" json:"new_params"`
	OldMetrics    json.RawMessage `gorm:"column:old_metrics;type:json;not null" json:"old_metrics"`
	NewMetrics    json.RawMessage `gorm:"column:new_metrics;type:json;not null" json:"new_metrics"`
	AnalysisNotes *string         `gorm:"column:analysis_notes;type:text" json:"analysis_notes"`
	Applied       bool            `gorm:"column:applied;not null;default:false" json:"applied"`
	CreatedAt     time.Time       `gorm:"column:created_at;autoCreateTime;index:idx_optimization_records_created_at" json:"created_at"`
}

// TableName overrides the default table name.
func (OptimizationRecord) TableName() string {
	return "optimization_records"
}
