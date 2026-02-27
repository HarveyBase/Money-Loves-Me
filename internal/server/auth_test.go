package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"pgregory.net/rapid"

	"money-loves-me/internal/model"
	"money-loves-me/internal/store"
)

func setupAuthTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	return db
}

func createTestUser(t testing.TB, us *store.UserStore, username, password string) {
	hash, err := HashPassword(password)
	require.NoError(t, err)
	require.NoError(t, us.Create(&model.User{
		Username:     username,
		PasswordHash: hash,
	}))
}

// Feature: binance-trading-system, Property 27: 账户锁定机制
// 连续3次登录失败后，账户将被锁定15分钟。
// 在锁定期间，即使输入正确密码也会被拒绝。
// **验证: 需求 12.5**
func TestProperty27_AccountLockoutMechanism(t *testing.T) {
	db := setupAuthTestDB(t)
	us := store.NewUserStore(db)
	auth := NewAuthService(us)

	rapid.Check(t, func(rt *rapid.T) {
		// 清理数据
		db.Exec("DELETE FROM users")

		username := rapid.StringMatching(`[a-z]{5,10}`).Draw(rt, "username")
		password := rapid.StringMatching(`[a-zA-Z0-9]{8,16}`).Draw(rt, "password")
		wrongPassword := password + "wrong"

		// 直接创建用户（避免传递 rapid.T）
		hash, err := HashPassword(password)
		require.NoError(t, err)
		require.NoError(t, us.Create(&model.User{Username: username, PasswordHash: hash}))

		// 验证初始登录正常
		resp, err := auth.Login(username, password)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// 为锁定测试重置状态
		db.Exec("UPDATE users SET failed_login_count = 0, locked_until = NULL WHERE username = ?", username)

		// 连续失败3次
		for i := 0; i < maxFailedAttempts; i++ {
			_, err := auth.Login(username, wrongPassword)
			assert.Error(t, err, "第 %d 次尝试应该失败", i+1)
		}

		// 账户现在应该被锁定
		locked, err := auth.IsAccountLocked(username)
		assert.NoError(t, err)
		assert.True(t, locked, "在 %d 次失败尝试后账户应该被锁定", maxFailedAttempts)

		// 在锁定期间即使正确密码也应被拒绝
		_, err = auth.Login(username, password)
		assert.Error(t, err, "在锁定期间即使密码正确也应该登录失败")
	})
}

func TestAuthService_LoginSuccess(t *testing.T) {
	db := setupAuthTestDB(t)
	us := store.NewUserStore(db)
	auth := NewAuthService(us)

	createTestUser(t, us, "testuser", "testpass123")

	resp, err := auth.Login("testuser", "testpass123")
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.True(t, resp.ExpiresAt.After(time.Now()))
}

func TestAuthService_LoginWrongPassword(t *testing.T) {
	db := setupAuthTestDB(t)
	us := store.NewUserStore(db)
	auth := NewAuthService(us)

	createTestUser(t, us, "testuser", "testpass123")

	_, err := auth.Login("testuser", "wrongpass")
	assert.Error(t, err)
}

func TestAuthService_LoginNonexistentUser(t *testing.T) {
	db := setupAuthTestDB(t)
	us := store.NewUserStore(db)
	auth := NewAuthService(us)

	_, err := auth.Login("nonexistent", "password")
	assert.Error(t, err)
}

func TestJWT_GenerateAndValidate(t *testing.T) {
	secret := []byte("test-secret")
	token, _, err := generateJWT(secret, "testuser")
	assert.NoError(t, err)

	username, err := validateJWT(secret, token)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", username)
}
