package store

import (
	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// RiskStore handles CRUD operations for risk configuration.
type RiskStore struct {
	*Store
}

// NewRiskStore creates a new RiskStore.
func NewRiskStore(db *gorm.DB) *RiskStore {
	return &RiskStore{Store: NewStore(db)}
}

// Get retrieves the latest (most recently updated) risk configuration.
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

// Save creates or updates the risk configuration.
// If the config has an ID, it updates; otherwise it creates a new record.
func (s *RiskStore) Save(config *model.RiskConfig) error {
	return s.withRetry(func() error {
		return s.db.Save(config).Error
	})
}
