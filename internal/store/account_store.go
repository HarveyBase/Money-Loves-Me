package store

import (
	"time"

	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// AccountStore 处理账户快照的 CRUD 操作。
type AccountStore struct {
	*Store
}

// NewAccountStore 创建一个新的 AccountStore。
func NewAccountStore(db *gorm.DB) *AccountStore {
	return &AccountStore{Store: NewStore(db)}
}

// Create 插入一条新的账户快照记录。
func (s *AccountStore) Create(snapshot *model.AccountSnapshot) error {
	return s.withRetry(func() error {
		return s.db.Create(snapshot).Error
	})
}

// GetByTimeRange 获取给定时间范围内的账户快照，
// 按快照时间降序排列。
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
