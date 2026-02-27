package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Store provides common database operations with retry logic.
type Store struct {
	db *gorm.DB
}

// NewStore creates a new Store instance.
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// DB returns the underlying gorm.DB instance.
func (s *Store) DB() *gorm.DB {
	return s.db
}

// withRetry executes fn with up to 3 retries using exponential backoff.
// Backoff delays: 100ms, 200ms, 400ms.
func (s *Store) withRetry(fn func() error) error {
	var err error
	for i := 0; i < 3; i++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(time.Duration(100<<uint(i)) * time.Millisecond)
	}
	return fmt.Errorf("operation failed after 3 retries: %w", err)
}

// TimeRangeQuery adds a time range condition to the query.
// Both start and end are inclusive.
func TimeRangeQuery(query *gorm.DB, field string, start, end time.Time) *gorm.DB {
	if !start.IsZero() {
		query = query.Where(fmt.Sprintf("%s >= ?", field), start)
	}
	if !end.IsZero() {
		query = query.Where(fmt.Sprintf("%s <= ?", field), end)
	}
	return query
}
