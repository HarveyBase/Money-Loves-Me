//go:build tools

package tools

// This file ensures Go module dependencies are tracked before they are used in code.
// These imports will be used by subsequent implementation tasks.
import (
	_ "github.com/gin-gonic/gin"
	_ "github.com/gorilla/websocket"
	_ "github.com/robfig/cron/v3"
	_ "github.com/shopspring/decimal"
	_ "github.com/spf13/viper"
	_ "go.uber.org/zap"
	_ "gorm.io/driver/mysql"
	_ "gorm.io/gorm"
	_ "pgregory.net/rapid"
)
