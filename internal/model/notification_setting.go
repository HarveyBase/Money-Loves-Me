package model

import (
	"encoding/json"
	"time"
)

// NotificationSetting represents the notification_settings table.
type NotificationSetting struct {
	ID            int             `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID        int             `gorm:"column:user_id;not null;index:idx_notification_settings_user_id" json:"user_id"`
	EnabledEvents json.RawMessage `gorm:"column:enabled_events;type:json;not null" json:"enabled_events"`
	UpdatedAt     time.Time       `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`

	// Belongs-to relationship
	User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE" json:"user,omitempty"`
}

// TableName overrides the default table name.
func (NotificationSetting) TableName() string {
	return "notification_settings"
}
