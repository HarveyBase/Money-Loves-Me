//go:build tools

package tools

// 此文件确保 Go 模块依赖在代码中使用之前被跟踪。
// 这些导入将被后续的实现任务使用。
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
