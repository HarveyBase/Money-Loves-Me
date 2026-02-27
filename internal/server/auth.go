package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"money-loves-me/internal/model"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxFailedAttempts = 3
	lockDuration      = 15 * time.Minute
	jwtSecret         = "trading-system-jwt-secret-key-2026" // In production, load from config
	jwtExpiry         = 24 * time.Hour
)

// UserStore abstracts user persistence.
type UserStore interface {
	GetByUsername(username string) (*model.User, error)
	Create(user *model.User) error
	Update(user *model.User) error
}

// AuthService handles user authentication.
type AuthService struct {
	userStore UserStore
	jwtSecret []byte
}

// NewAuthService creates a new AuthService.
func NewAuthService(us UserStore) *AuthService {
	return &AuthService{
		userStore: us,
		jwtSecret: []byte(jwtSecret),
	}
}

// LoginRequest represents a login request body.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a successful login response.
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Login authenticates a user and returns a JWT token.
func (a *AuthService) Login(username, password string) (*LoginResponse, error) {
	user, err := a.userStore.GetByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	// Check if account is locked
	if user.LockedUntil.Valid && time.Now().Before(user.LockedUntil.Time) {
		return nil, fmt.Errorf("account locked until %s", user.LockedUntil.Time.Format(time.RFC3339))
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Increment failed login count
		user.FailedLoginCount++
		if user.FailedLoginCount >= maxFailedAttempts {
			user.LockedUntil = sql.NullTime{Time: time.Now().Add(lockDuration), Valid: true}
		}
		a.userStore.Update(user)
		return nil, fmt.Errorf("invalid username or password")
	}

	// Reset failed login count on success
	user.FailedLoginCount = 0
	user.LockedUntil = sql.NullTime{Valid: false}
	a.userStore.Update(user)

	// Generate JWT token
	token, expiresAt, err := generateJWT(a.jwtSecret, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginResponse{Token: token, ExpiresAt: expiresAt}, nil
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// IsAccountLocked checks if a user account is currently locked.
func (a *AuthService) IsAccountLocked(username string) (bool, error) {
	user, err := a.userStore.GetByUsername(username)
	if err != nil {
		return false, err
	}
	return user.LockedUntil.Valid && time.Now().Before(user.LockedUntil.Time), nil
}

// JWTAuthMiddleware creates a Gin middleware that validates JWT tokens.
func (a *AuthService) JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		username, err := validateJWT(a.jwtSecret, parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("username", username)
		c.Next()
	}
}
