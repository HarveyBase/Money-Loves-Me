package store

import (
	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// OptimizationStore handles CRUD operations for optimization records.
type OptimizationStore struct {
	*Store
}

// NewOptimizationStore creates a new OptimizationStore.
func NewOptimizationStore(db *gorm.DB) *OptimizationStore {
	return &OptimizationStore{Store: NewStore(db)}
}

// Create inserts a new optimization record.
func (s *OptimizationStore) Create(record *model.OptimizationRecord) error {
	return s.withRetry(func() error {
		return s.db.Create(record).Error
	})
}

// GetByStrategy retrieves all optimization records for a given strategy name,
// ordered by creation time descending.
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

// GetAll retrieves all optimization records ordered by creation time descending.
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
