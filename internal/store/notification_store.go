package store

import (
	"time"

	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// NotificationStore handles CRUD operations for notifications.
type NotificationStore struct {
	*Store
}

// NewNotificationStore creates a new NotificationStore.
func NewNotificationStore(db *gorm.DB) *NotificationStore {
	return &NotificationStore{Store: NewStore(db)}
}

// Create inserts a new notification record.
func (s *NotificationStore) Create(notification *model.Notification) error {
	return s.withRetry(func() error {
		return s.db.Create(notification).Error
	})
}

// NotificationFilter defines filtering criteria for notification queries.
type NotificationFilter struct {
	EventType string
	IsRead    *bool
	Start     time.Time
	End       time.Time
}

// GetByFilter retrieves notifications matching the given filter criteria,
// ordered by creation time descending.
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

// MarkAsRead sets is_read = true for the notification with the given ID.
func (s *NotificationStore) MarkAsRead(id int64) error {
	return s.withRetry(func() error {
		return s.db.Model(&model.Notification{}).Where("id = ?", id).Update("is_read", true).Error
	})
}
