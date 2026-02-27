package store

import (
	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// UserStore 处理用户的 CRUD 操作。
type UserStore struct {
	*Store
}

// NewUserStore 创建一个新的 UserStore。
func NewUserStore(db *gorm.DB) *UserStore {
	return &UserStore{Store: NewStore(db)}
}

// Create 插入一条新的用户记录。
func (s *UserStore) Create(user *model.User) error {
	return s.withRetry(func() error {
		return s.db.Create(user).Error
	})
}

// GetByUsername 根据唯一用户名获取用户。
func (s *UserStore) GetByUsername(username string) (*model.User, error) {
	var user model.User
	err := s.withRetry(func() error {
		return s.db.Where("username = ?", username).First(&user).Error
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update 保存对现有用户的更改。
func (s *UserStore) Update(user *model.User) error {
	return s.withRetry(func() error {
		return s.db.Save(user).Error
	})
}
