package store

import (
	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// StrategyStore 处理策略的 CRUD 操作。
type StrategyStore struct {
	*Store
}

// NewStrategyStore 创建一个新的 StrategyStore。
func NewStrategyStore(db *gorm.DB) *StrategyStore {
	return &StrategyStore{Store: NewStore(db)}
}

// Create 插入一条新的策略记录。
func (s *StrategyStore) Create(strategy *model.Strategy) error {
	return s.withRetry(func() error {
		return s.db.Create(strategy).Error
	})
}

// GetByName 根据唯一名称获取策略。
func (s *StrategyStore) GetByName(name string) (*model.Strategy, error) {
	var strategy model.Strategy
	err := s.withRetry(func() error {
		return s.db.Where("name = ?", name).First(&strategy).Error
	})
	if err != nil {
		return nil, err
	}
	return &strategy, nil
}

// Update 保存对现有策略的更改。
func (s *StrategyStore) Update(strategy *model.Strategy) error {
	return s.withRetry(func() error {
		return s.db.Save(strategy).Error
	})
}

// GetAll 获取所有策略。
func (s *StrategyStore) GetAll() ([]model.Strategy, error) {
	var strategies []model.Strategy
	err := s.withRetry(func() error {
		return s.db.Find(&strategies).Error
	})
	if err != nil {
		return nil, err
	}
	return strategies, nil
}

// GetActive 获取所有已启用的策略（active = true）。
func (s *StrategyStore) GetActive() ([]model.Strategy, error) {
	var strategies []model.Strategy
	err := s.withRetry(func() error {
		return s.db.Where("active = ?", true).Find(&strategies).Error
	})
	if err != nil {
		return nil, err
	}
	return strategies, nil
}
