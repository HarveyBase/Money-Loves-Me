package store

import (
	"money-loves-me/internal/model"

	"gorm.io/gorm"
)

// UserStore handles CRUD operations for users.
type UserStore struct {
	*Store
}

// NewUserStore creates a new UserStore.
func NewUserStore(db *gorm.DB) *UserStore {
	return &UserStore{Store: NewStore(db)}
}

// Create inserts a new user record.
func (s *UserStore) Create(user *model.User) error {
	return s.withRetry(func() error {
		return s.db.Create(user).Error
	})
}

// GetByUsername retrieves a user by their unique username.
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

// Update saves changes to an existing user.
func (s *UserStore) Update(user *model.User) error {
	return s.withRetry(func() error {
		return s.db.Save(user).Error
	})
}
