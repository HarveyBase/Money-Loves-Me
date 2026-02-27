package config

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

// Config 是交易系统的根配置结构体。
type Config struct {
	Server    ServerConfig    `yaml:"server"    mapstructure:"server"`
	Binance   BinanceConfig   `yaml:"binance"   mapstructure:"binance"`
	Database  DatabaseConfig  `yaml:"database"  mapstructure:"database"`
	Log       LogConfig       `yaml:"log"       mapstructure:"log"`
	Trading   TradingConfig   `yaml:"trading"   mapstructure:"trading"`
	Risk      RiskConfig      `yaml:"risk"      mapstructure:"risk"`
	Optimizer OptimizerConfig `yaml:"optimizer" mapstructure:"optimizer"`
}

// ServerConfig 保存 HTTP 服务器设置。
type ServerConfig struct {
	Host string `yaml:"host" mapstructure:"host"`
	Port int    `yaml:"port" mapstructure:"port"`
	Mode string `yaml:"mode" mapstructure:"mode"` // debug, release, test
}

// BinanceConfig 保存币安 API 连接设置。
type BinanceConfig struct {
	APIKey    string `yaml:"api_key"    mapstructure:"api_key"`    // AES-256 加密
	SecretKey string `yaml:"secret_key" mapstructure:"secret_key"` // AES-256 加密
	BaseURL   string `yaml:"base_url"   mapstructure:"base_url"`
	WsURL     string `yaml:"ws_url"     mapstructure:"ws_url"`
}

// DatabaseConfig 保存数据库连接设置，支持 MySQL 和 SQLite。
type DatabaseConfig struct {
	Driver   string `yaml:"driver"   mapstructure:"driver"` // mysql 或 sqlite，默认 mysql
	Host     string `yaml:"host"     mapstructure:"host"`
	Port     int    `yaml:"port"     mapstructure:"port"`
	User     string `yaml:"user"     mapstructure:"user"`
	Password string `yaml:"password" mapstructure:"password"` // AES-256 加密
	DBName   string `yaml:"db_name"  mapstructure:"db_name"`
}

// LogConfig 保存日志设置。
type LogConfig struct {
	Level      string `yaml:"level"       mapstructure:"level"` // DEBUG, INFO, WARN, ERROR
	FilePath   string `yaml:"file_path"   mapstructure:"file_path"`
	MaxSizeMB  int    `yaml:"max_size_mb" mapstructure:"max_size_mb"`   // 单个文件最大大小（MB）
	MaxAgeDays int    `yaml:"max_age_days" mapstructure:"max_age_days"` // 保留天数
}

// TradingConfig 保存默认交易设置。
type TradingConfig struct {
	DefaultPairs []string `yaml:"default_pairs" mapstructure:"default_pairs"` // 例如 ["BTCUSDT", "ETHUSDT"]
}

// RiskConfig 保存风控管理设置。
type RiskConfig struct {
	MaxOrderAmount     decimal.Decimal            `yaml:"max_order_amount"     mapstructure:"max_order_amount"`
	MaxDailyLoss       decimal.Decimal            `yaml:"max_daily_loss"       mapstructure:"max_daily_loss"`
	StopLossPercent    map[string]decimal.Decimal `yaml:"stop_loss_percent"    mapstructure:"stop_loss_percent"`
	MaxPositionPercent map[string]decimal.Decimal `yaml:"max_position_percent" mapstructure:"max_position_percent"`
}

// OptimizerConfig 保存策略优化器设置。
type OptimizerConfig struct {
	Interval       time.Duration `yaml:"interval"         mapstructure:"interval"`
	LookbackDays   int           `yaml:"lookback_days"    mapstructure:"lookback_days"`
	MaxParamChange float64       `yaml:"max_param_change" mapstructure:"max_param_change"` // 0.3 = 30%
}

// Load 使用 Viper 从给定的 YAML 文件路径读取配置。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	decoderOpt := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		stringToDecimalHookFunc(),
	))
	if err := v.Unmarshal(&cfg, decoderOpt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// stringToDecimalHookFunc 返回一个 mapstructure 解码钩子，
// 用于将字符串值转换为 shopspring/decimal.Decimal。
func stringToDecimalHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(decimal.Decimal{}) {
			return data, nil
		}
		switch v := data.(type) {
		case string:
			return decimal.NewFromString(v)
		case float64:
			return decimal.NewFromFloat(v), nil
		case int64:
			return decimal.NewFromInt(v), nil
		case int:
			return decimal.NewFromInt(int64(v)), nil
		default:
			return data, nil
		}
	}
}

// Validate 检查所有必填字段是否存在且有效。
func (c *Config) Validate() error {
	var errs []string

	// 服务器验证
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errs = append(errs, "server.port must be between 1 and 65535")
	}

	// 币安验证
	if c.Binance.APIKey == "" {
		errs = append(errs, "binance.api_key is required")
	}
	if c.Binance.SecretKey == "" {
		errs = append(errs, "binance.secret_key is required")
	}
	if c.Binance.BaseURL == "" {
		errs = append(errs, "binance.base_url is required")
	}
	if c.Binance.WsURL == "" {
		errs = append(errs, "binance.ws_url is required")
	}

	// 数据库验证（SQLite 模式下跳过 MySQL 相关验证）
	if c.Database.Driver != "sqlite" {
		if c.Database.Host == "" {
			errs = append(errs, "database.host is required")
		}
		if c.Database.Port <= 0 || c.Database.Port > 65535 {
			errs = append(errs, "database.port must be between 1 and 65535")
		}
		if c.Database.User == "" {
			errs = append(errs, "database.user is required")
		}
		if c.Database.Password == "" {
			errs = append(errs, "database.password is required")
		}
		if c.Database.DBName == "" {
			errs = append(errs, "database.db_name is required")
		}
	}

	// 日志验证
	validLogLevels := map[string]bool{"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true}
	if !validLogLevels[strings.ToUpper(c.Log.Level)] {
		errs = append(errs, "log.level must be one of: DEBUG, INFO, WARN, ERROR")
	}
	if c.Log.MaxSizeMB <= 0 {
		errs = append(errs, "log.max_size_mb must be positive")
	}
	if c.Log.MaxAgeDays <= 0 {
		errs = append(errs, "log.max_age_days must be positive")
	}

	// 交易验证
	if len(c.Trading.DefaultPairs) == 0 {
		errs = append(errs, "trading.default_pairs must contain at least one pair")
	}

	// 风控验证
	if c.Risk.MaxOrderAmount.IsNegative() || c.Risk.MaxOrderAmount.IsZero() {
		errs = append(errs, "risk.max_order_amount must be positive")
	}
	if c.Risk.MaxDailyLoss.IsNegative() || c.Risk.MaxDailyLoss.IsZero() {
		errs = append(errs, "risk.max_daily_loss must be positive")
	}

	// 优化器验证
	if c.Optimizer.Interval <= 0 {
		errs = append(errs, "optimizer.interval must be positive")
	}
	if c.Optimizer.LookbackDays <= 0 {
		errs = append(errs, "optimizer.lookback_days must be positive")
	}
	if c.Optimizer.MaxParamChange <= 0 || c.Optimizer.MaxParamChange > 1.0 {
		errs = append(errs, "optimizer.max_param_change must be between 0 and 1")
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
