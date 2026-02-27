package store

import (
	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// StrategyStore handles CRUD operations for strategies.
type StrategyStore struct {
	*Store
}

// NewStrategyStore creates a new StrategyStore.
func NewStrategyStore(db *gorm.DB) *StrategyStore {
	return &StrategyStore{Store: NewStore(db)}
}

// Create inserts a new strategy record.
func (s *StrategyStore) Create(strategy *model.Strategy) error {
	return s.withRetry(func() error {
		return s.db.Create(strategy).Error
	})
}

// GetByName retrieves a strategy by its unique name.
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

// Update saves changes to an existing strategy.
func (s *StrategyStore) Update(strategy *model.Strategy) error {
	return s.withRetry(func() error {
		return s.db.Save(strategy).Error
	})
}

// GetAll retrieves all strategies.
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

// GetActive retrieves all strategies where active = true.
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
