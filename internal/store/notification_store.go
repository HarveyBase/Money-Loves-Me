package store

import (
	"time"

	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// NotificationStore 处理通知的 CRUD 操作。
type NotificationStore struct {
	*Store
}

// NewNotificationStore 创建一个新的 NotificationStore。
func NewNotificationStore(db *gorm.DB) *NotificationStore {
	return &NotificationStore{Store: NewStore(db)}
}

// Create 插入一条新的通知记录。
func (s *NotificationStore) Create(notification *model.Notification) error {
	return s.withRetry(func() error {
		return s.db.Create(notification).Error
	})
}

// NotificationFilter 定义通知查询的过滤条件。
type NotificationFilter struct {
	EventType string
	IsRead    *bool
	Start     time.Time
	End       time.Time
}

// GetByFilter 获取符合给定过滤条件的通知，
// 按创建时间降序排列。
func (s *NotificationStore) GetByFilter(filter NotificationFilter) ([]model.Notification, error) {
	var notifications []model.Notification
	err := s.withRetry(func() error {
		query := s.db.Model(&model.Notification{})
		if filter.EventType != "" {
			query = query.Where("event_type = ?", filter.EventType)
		}
		if filter.IsRead != nil {
			query = query.Where("is_read = ?", *filter.IsRead)
		}
		query = TimeRangeQuery(query, "created_at", filter.Start, filter.End)
		return query.Order("created_at DESC").Find(&notifications).Error
	})
	if err != nil {
		return nil, err
	}
	return notifications, nil
}

// MarkAsRead 将指定 ID 的通知标记为已读（is_read = true）。
func (s *NotificationStore) MarkAsRead(id int64) error {
	return s.withRetry(func() error {
		return s.db.Model(&model.Notification{}).Where("id = ?", id).Update("is_read", true).Error
	})
}
