package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Store 提供带有重试逻辑的通用数据库操作。
type Store struct {
	db *gorm.DB
}

// NewStore 创建一个新的 Store 实例。
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// DB 返回底层的 gorm.DB 实例。
func (s *Store) DB() *gorm.DB {
	return s.db
}

// withRetry 使用指数退避策略执行 fn，最多重试 3 次。
// 退避延迟：100ms、200ms、400ms。
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

// TimeRangeQuery 为查询添加时间范围条件。
// start 和 end 均为闭区间（包含边界值）。
func TimeRangeQuery(query *gorm.DB, field string, start, end time.Time) *gorm.DB {
	if !start.IsZero() {
		query = query.Where(fmt.Sprintf("%s >= ?", field), start)
	}
	if !end.IsZero() {
		query = query.Where(fmt.Sprintf("%s <= ?", field), end)
	}
	return query
}
