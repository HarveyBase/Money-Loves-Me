package model

import (
	"encoding/json"
	"time"
)

// NotificationSetting 表示 notification_settings 数据表。
type NotificationSetting struct {
	ID            int             `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID        int             `gorm:"column:user_id;not null;index:idx_notification_settings_user_id" json:"user_id"`
	EnabledEvents json.RawMessage `gorm:"column:enabled_events;type:json;not null" json:"enabled_events"`
	UpdatedAt     time.Time       `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`

	// 属于（Belongs-to）关联关系
	User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE" json:"user,omitempty"`
}

// TableName 覆盖默认的表名。
func (NotificationSetting) TableName() string {
	return "notification_settings"
}
