package store

import (
	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// BacktestStore handles CRUD operations for backtest results.
type BacktestStore struct {
	*Store
}

// NewBacktestStore creates a new BacktestStore.
func NewBacktestStore(db *gorm.DB) *BacktestStore {
	return &BacktestStore{Store: NewStore(db)}
}

// Create inserts a new backtest result record.
func (s *BacktestStore) Create(result *model.BacktestResult) error {
	return s.withRetry(func() error {
		return s.db.Create(result).Error
	})
}

// GetByID retrieves a backtest result by its primary key.
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

// GetByStrategy retrieves all backtest results for a given strategy name,
// ordered by creation time descending.
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
