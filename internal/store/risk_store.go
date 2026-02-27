package store

import (
	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// RiskStore 处理风控配置的 CRUD 操作。
type RiskStore struct {
	*Store
}

// NewRiskStore 创建一个新的 RiskStore。
func NewRiskStore(db *gorm.DB) *RiskStore {
	return &RiskStore{Store: NewStore(db)}
}

// Get 获取最新的（最近更新的）风控配置。
func (s *RiskStore) Get() (*model.RiskConfig, error) {
	var config model.RiskConfig
	err := s.withRetry(func() error {
		return s.db.Order("updated_at DESC").First(&config).Error
	})
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// Save 创建或更新风控配置。
// 如果配置有 ID，则更新；否则创建新记录。
func (s *RiskStore) Save(config *model.RiskConfig) error {
	return s.withRetry(func() error {
		return s.db.Save(config).Error
	})
}
