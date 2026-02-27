package store

import (
	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// BacktestStore 处理回测结果的 CRUD 操作。
type BacktestStore struct {
	*Store
}

// NewBacktestStore 创建一个新的 BacktestStore。
func NewBacktestStore(db *gorm.DB) *BacktestStore {
	return &BacktestStore{Store: NewStore(db)}
}

// Create 插入一条新的回测结果记录。
func (s *BacktestStore) Create(result *model.BacktestResult) error {
	return s.withRetry(func() error {
		return s.db.Create(result).Error
	})
}

// GetByID 根据主键获取回测结果。
func (s *BacktestStore) GetByID(id int64) (*model.BacktestResult, error) {
	var result model.BacktestResult
	err := s.withRetry(func() error {
		return s.db.First(&result, id).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetByStrategy 获取指定策略名称的所有回测结果，
// 按创建时间降序排列。
func (s *BacktestStore) GetByStrategy(strategyName string) ([]model.BacktestResult, error) {
	var results []model.BacktestResult
	err := s.withRetry(func() error {
		return s.db.Where("strategy_name = ?", strategyName).
			Order("created_at DESC").
			Find(&results).Error
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}
