package store

import (
	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// OptimizationStore 处理优化记录的 CRUD 操作。
type OptimizationStore struct {
	*Store
}

// NewOptimizationStore 创建一个新的 OptimizationStore。
func NewOptimizationStore(db *gorm.DB) *OptimizationStore {
	return &OptimizationStore{Store: NewStore(db)}
}

// Create 插入一条新的优化记录。
func (s *OptimizationStore) Create(record *model.OptimizationRecord) error {
	return s.withRetry(func() error {
		return s.db.Create(record).Error
	})
}

// GetByStrategy 获取指定策略名称的所有优化记录，
// 按创建时间降序排列。
func (s *OptimizationStore) GetByStrategy(strategyName string) ([]model.OptimizationRecord, error) {
	var records []model.OptimizationRecord
	err := s.withRetry(func() error {
		return s.db.Where("strategy_name = ?", strategyName).
			Order("created_at DESC").
			Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GetAll 获取所有优化记录，按创建时间降序排列。
func (s *OptimizationStore) GetAll() ([]model.OptimizationRecord, error) {
	var records []model.OptimizationRecord
	err := s.withRetry(func() error {
		return s.db.Order("created_at DESC").Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}
