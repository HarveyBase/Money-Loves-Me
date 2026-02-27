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

// Config is the root configuration structure for the trading system.
type Config struct {
	Server    ServerConfig    `yaml:"server"    mapstructure:"server"`
	Binance   BinanceConfig   `yaml:"binance"   mapstructure:"binance"`
	Database  DatabaseConfig  `yaml:"database"  mapstructure:"database"`
	Log       LogConfig       `yaml:"log"       mapstructure:"log"`
	Trading   TradingConfig   `yaml:"trading"   mapstructure:"trading"`
	Risk      RiskConfig      `yaml:"risk"      mapstructure:"risk"`
	Optimizer OptimizerConfig `yaml:"optimizer" mapstructure:"optimizer"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `yaml:"host" mapstructure:"host"`
	Port int    `yaml:"port" mapstructure:"port"`
	Mode string `yaml:"mode" mapstructure:"mode"` // debug, release, test
}

// BinanceConfig holds Binance API connection settings.
type BinanceConfig struct {
	APIKey    string `yaml:"api_key"    mapstructure:"api_key"`    // AES-256 encrypted
	SecretKey string `yaml:"secret_key" mapstructure:"secret_key"` // AES-256 encrypted
	BaseURL   string `yaml:"base_url"   mapstructure:"base_url"`
	WsURL     string `yaml:"ws_url"     mapstructure:"ws_url"`
}

// DatabaseConfig holds MySQL connection settings.
type DatabaseConfig struct {
	Host     string `yaml:"host"     mapstructure:"host"`
	Port     int    `yaml:"port"     mapstructure:"port"`
	User     string `yaml:"user"     mapstructure:"user"`
	Password string `yaml:"password" mapstructure:"password"` // AES-256 encrypted
	DBName   string `yaml:"db_name"  mapstructure:"db_name"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level      string `yaml:"level"       mapstructure:"level"` // DEBUG, INFO, WARN, ERROR
	FilePath   string `yaml:"file_path"   mapstructure:"file_path"`
	MaxSizeMB  int    `yaml:"max_size_mb" mapstructure:"max_size_mb"`   // max single file size in MB
	MaxAgeDays int    `yaml:"max_age_days" mapstructure:"max_age_days"` // retention in days
}

// TradingConfig holds default trading settings.
type TradingConfig struct {
	DefaultPairs []string `yaml:"default_pairs" mapstructure:"default_pairs"` // e.g. ["BTCUSDT", "ETHUSDT"]
}

// RiskConfig holds risk management settings.
type RiskConfig struct {
	MaxOrderAmount     decimal.Decimal            `yaml:"max_order_amount"     mapstructure:"max_order_amount"`
	MaxDailyLoss       decimal.Decimal            `yaml:"max_daily_loss"       mapstructure:"max_daily_loss"`
	StopLossPercent    map[string]decimal.Decimal `yaml:"stop_loss_percent"    mapstructure:"stop_loss_percent"`
	MaxPositionPercent map[string]decimal.Decimal `yaml:"max_position_percent" mapstructure:"max_position_percent"`
}

// OptimizerConfig holds strategy optimizer settings.
type OptimizerConfig struct {
	Interval       time.Duration `yaml:"interval"         mapstructure:"interval"`
	LookbackDays   int           `yaml:"lookback_days"    mapstructure:"lookback_days"`
	MaxParamChange float64       `yaml:"max_param_change" mapstructure:"max_param_change"` // 0.3 = 30%
}

// Load reads the configuration from the given YAML file path using Viper.
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

// stringToDecimalHookFunc returns a mapstructure decode hook that converts
// string values to shopspring/decimal.Decimal.
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

// Validate checks that all required fields are present and valid.
func (c *Config) Validate() error {
	var errs []string

	// Server validation
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errs = append(errs, "server.port must be between 1 and 65535")
	}

	// Binance validation
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

	// Database validation
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

	// Log validation
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

	// Trading validation
	if len(c.Trading.DefaultPairs) == 0 {
		errs = append(errs, "trading.default_pairs must contain at least one pair")
	}

	// Risk validation
	if c.Risk.MaxOrderAmount.IsNegative() || c.Risk.MaxOrderAmount.IsZero() {
		errs = append(errs, "risk.max_order_amount must be positive")
	}
	if c.Risk.MaxDailyLoss.IsNegative() || c.Risk.MaxDailyLoss.IsZero() {
		errs = append(errs, "risk.max_daily_loss must be positive")
	}

	// Optimizer validation
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
