package model

import (
	"database/sql"
	"time"
)

// User represents the users table.
type User struct {
	ID               int          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Username         string       `gorm:"column:username;type:varchar(64);not null;uniqueIndex:uk_users_username" json:"username"`
	PasswordHash     string       `gorm:"column:password_hash;type:varchar(255);not null" json:"-"`
	FailedLoginCount int          `gorm:"column:failed_login_count;not null;default:0" json:"failed_login_count"`
	LockedUntil      sql.NullTime `gorm:"column:locked_until" json:"locked_until"`
	CreatedAt        time.Time    `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

// TableName overrides the default table name.
func (User) TableName() string {
	return "users"
}
