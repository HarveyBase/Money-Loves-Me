package model

import "time"

// Notification represents the notifications table.
type Notification struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	EventType   string    `gorm:"column:event_type;type:varchar(30);not null;index:idx_notifications_event_type" json:"event_type"`
	Title       string    `gorm:"column:title;type:varchar(255);not null" json:"title"`
	Description *string   `gorm:"column:description;type:text" json:"description"`
	IsRead      bool      `gorm:"column:is_read;not null;default:false;index:idx_notifications_is_read" json:"is_read"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime;index:idx_notifications_created_at" json:"created_at"`
}

// TableName overrides the default table name.
func (Notification) TableName() string {
	return "notifications"
}
