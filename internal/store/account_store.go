package store

import (
	"time"

	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// AccountStore handles CRUD operations for account snapshots.
type AccountStore struct {
	*Store
}

// NewAccountStore creates a new AccountStore.
func NewAccountStore(db *gorm.DB) *AccountStore {
	return &AccountStore{Store: NewStore(db)}
}

// Create inserts a new account snapshot record.
func (s *AccountStore) Create(snapshot *model.AccountSnapshot) error {
	return s.withRetry(func() error {
		return s.db.Create(snapshot).Error
	})
}

// GetByTimeRange retrieves account snapshots within the given time range,
// ordered by snapshot time descending.
func (s *AccountStore) GetByTimeRange(start, end time.Time) ([]model.AccountSnapshot, error) {
	var snapshots []model.AccountSnapshot
	err := s.withRetry(func() error {
		query := s.db.Model(&model.AccountSnapshot{})
		query = TimeRangeQuery(query, "snapshot_at", start, end)
		return query.Order("snapshot_at DESC").Find(&snapshots).Error
	})
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}
