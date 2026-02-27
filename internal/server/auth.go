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
	jwtSecret         = "trading-system-jwt-secret-key-2026" // 生产环境中应从配置文件加载
	jwtExpiry         = 24 * time.Hour
)

// UserStore 抽象用户持久化存储接口。
type UserStore interface {
	GetByUsername(username string) (*model.User, error)
	Create(user *model.User) error
	Update(user *model.User) error
}

// AuthService 处理用户认证。
type AuthService struct {
	userStore UserStore
	jwtSecret []byte
}

// NewAuthService 创建一个新的 AuthService。
func NewAuthService(us UserStore) *AuthService {
	return &AuthService{
		userStore: us,
		jwtSecret: []byte(jwtSecret),
	}
}

// LoginRequest 表示登录请求体。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 表示登录成功的响应。
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Login 验证用户身份并返回 JWT 令牌。
func (a *AuthService) Login(username, password string) (*LoginResponse, error) {
	user, err := a.userStore.GetByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	// 检查账户是否被锁定
	if user.LockedUntil.Valid && time.Now().Before(user.LockedUntil.Time) {
		return nil, fmt.Errorf("account locked until %s", user.LockedUntil.Time.Format(time.RFC3339))
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// 增加登录失败计数
		user.FailedLoginCount++
		if user.FailedLoginCount >= maxFailedAttempts {
			user.LockedUntil = sql.NullTime{Time: time.Now().Add(lockDuration), Valid: true}
		}
		a.userStore.Update(user)
		return nil, fmt.Errorf("invalid username or password")
	}

	// 登录成功后重置失败计数
	user.FailedLoginCount = 0
	user.LockedUntil = sql.NullTime{Valid: false}
	a.userStore.Update(user)

	// 生成 JWT 令牌
	token, expiresAt, err := generateJWT(a.jwtSecret, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginResponse{Token: token, ExpiresAt: expiresAt}, nil
}

// HashPassword 使用 bcrypt 对密码进行哈希处理。
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// IsAccountLocked 检查用户账户当前是否被锁定。
func (a *AuthService) IsAccountLocked(username string) (bool, error) {
	user, err := a.userStore.GetByUsername(username)
	if err != nil {
		return false, err
	}
	return user.LockedUntil.Valid && time.Now().Before(user.LockedUntil.Time), nil
}

// JWTAuthMiddleware 创建一个验证 JWT 令牌的 Gin 中间件。
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
