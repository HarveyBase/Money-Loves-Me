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
// After 3 consecutive failed login attempts, the account is locked for 15 minutes.
// During lockout, even correct passwords are rejected.
// **Validates: Requirements 12.5**
func TestProperty27_AccountLockoutMechanism(t *testing.T) {
	db := setupAuthTestDB(t)
	us := store.NewUserStore(db)
	auth := NewAuthService(us)

	rapid.Check(t, func(rt *rapid.T) {
		// Clean up
		db.Exec("DELETE FROM users")

		username := rapid.StringMatching(`[a-z]{5,10}`).Draw(rt, "username")
		password := rapid.StringMatching(`[a-zA-Z0-9]{8,16}`).Draw(rt, "password")
		wrongPassword := password + "wrong"

		// Create user directly (avoid passing rapid.T)
		hash, err := HashPassword(password)
		require.NoError(t, err)
		require.NoError(t, us.Create(&model.User{Username: username, PasswordHash: hash}))

		// Verify login works initially
		resp, err := auth.Login(username, password)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Reset for the lockout test
		db.Exec("UPDATE users SET failed_login_count = 0, locked_until = NULL WHERE username = ?", username)

		// Fail 3 times consecutively
		for i := 0; i < maxFailedAttempts; i++ {
			_, err := auth.Login(username, wrongPassword)
			assert.Error(t, err, "attempt %d should fail", i+1)
		}

		// Account should now be locked
		locked, err := auth.IsAccountLocked(username)
		assert.NoError(t, err)
		assert.True(t, locked, "account should be locked after %d failed attempts", maxFailedAttempts)

		// Even correct password should be rejected during lockout
		_, err = auth.Login(username, password)
		assert.Error(t, err, "login should fail during lockout even with correct password")
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
