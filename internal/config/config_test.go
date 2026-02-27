package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"
binance:
  api_key: "test-key"
  secret_key: "test-secret"
  base_url: "https://api.binance.com"
  ws_url: "wss://stream.binance.com:9443"
database:
  host: "127.0.0.1"
  port: 3306
  user: "root"
  password: "pass"
  db_name: "testdb"
log:
  level: "INFO"
  file_path: "logs/test.log"
  max_size_mb: 100
  max_age_days: 30
trading:
  default_pairs:
    - "BTCUSDT"
risk:
  max_order_amount: "1000"
  max_daily_loss: "500"
  stop_loss_percent:
    BTCUSDT: "0.05"
  max_position_percent:
    BTCUSDT: "0.3"
optimizer:
  interval: "24h"
  lookback_days: 30
  max_param_change: 0.3
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Binance.APIKey != "test-key" {
		t.Errorf("expected api_key 'test-key', got %q", cfg.Binance.APIKey)
	}
	if cfg.Database.DBName != "testdb" {
		t.Errorf("expected db_name 'testdb', got %q", cfg.Database.DBName)
	}
	if cfg.Optimizer.Interval != 24*time.Hour {
		t.Errorf("expected interval 24h, got %v", cfg.Optimizer.Interval)
	}
	if !cfg.Risk.MaxOrderAmount.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("expected max_order_amount 1000, got %s", cfg.Risk.MaxOrderAmount)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidate_MissingRequiredFields(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty config")
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := validConfig()
	cfg.Server.Port = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for port 0")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := validConfig()
	cfg.Log.Level = "INVALID"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid log level")
	}
}

func TestValidate_EmptyAPIKey(t *testing.T) {
	cfg := validConfig()
	cfg.Binance.APIKey = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty api_key")
	}
}

func TestValidate_NegativeMaxOrderAmount(t *testing.T) {
	cfg := validConfig()
	cfg.Risk.MaxOrderAmount = decimal.NewFromInt(-1)
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for negative max_order_amount")
	}
}

func TestValidate_InvalidOptimizerMaxParamChange(t *testing.T) {
	cfg := validConfig()
	cfg.Optimizer.MaxParamChange = 1.5
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for max_param_change > 1")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// validConfig 返回一个完整填充的有效 Config，用于测试。
func validConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
			Mode: "release",
		},
		Binance: BinanceConfig{
			APIKey:    "test-key",
			SecretKey: "test-secret",
			BaseURL:   "https://api.binance.com",
			WsURL:     "wss://stream.binance.com:9443",
		},
		Database: DatabaseConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "pass",
			DBName:   "testdb",
		},
		Log: LogConfig{
			Level:      "INFO",
			FilePath:   "logs/test.log",
			MaxSizeMB:  100,
			MaxAgeDays: 30,
		},
		Trading: TradingConfig{
			DefaultPairs: []string{"BTCUSDT"},
		},
		Risk: RiskConfig{
			MaxOrderAmount:     decimal.NewFromInt(1000),
			MaxDailyLoss:       decimal.NewFromInt(500),
			StopLossPercent:    map[string]decimal.Decimal{"BTCUSDT": decimal.NewFromFloat(0.05)},
			MaxPositionPercent: map[string]decimal.Decimal{"BTCUSDT": decimal.NewFromFloat(0.3)},
		},
		Optimizer: OptimizerConfig{
			Interval:       24 * time.Hour,
			LookbackDays:   30,
			MaxParamChange: 0.3,
		},
	}
}

// Feature: binance-trading-system, Property 25: YAML 配置解析往返
// **Validates: Requirements 12.1**
// Property 25: 对于任意有效配置结构体，序列化为 YAML 后再反序列化应得到等价结构体
func TestProperty25_YAMLConfigRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 使用 rapid 生成器生成一个有效的 Config
		cfg := drawValidConfig(t)

		// 序列化为 YAML 兼容的 map 并写入临时文件
		yamlContent := configToYAML(cfg)

		dir := filepath.Join(os.TempDir(), "config_pbt_"+fmt.Sprintf("%d", time.Now().UnixNano()))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		cfgPath := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
			t.Fatal(err)
		}

		// 使用 Load() 函数（基于 Viper）重新加载
		loaded, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load failed: %v\nYAML content:\n%s", err, yamlContent)
		}

		// 逐字段比较
		assertConfigEqual(t, cfg, loaded)
	})
}

// drawValidConfig 使用 rapid 生成器生成一个随机的有效 Config。
func drawValidConfig(t *rapid.T) *Config {
	// ServerConfig
	host := rapid.StringMatching(`[a-z0-9]{1,10}(\.[a-z0-9]{1,10})*`).Draw(t, "server.host")
	port := rapid.IntRange(1, 65535).Draw(t, "server.port")
	mode := rapid.SampledFrom([]string{"debug", "release", "test"}).Draw(t, "server.mode")

	// BinanceConfig - 不含 YAML 特殊字符的非空字符串
	apiKey := rapid.StringMatching(`[a-zA-Z0-9]{1,32}`).Draw(t, "binance.api_key")
	secretKey := rapid.StringMatching(`[a-zA-Z0-9]{1,32}`).Draw(t, "binance.secret_key")
	baseURL := "https://" + rapid.StringMatching(`[a-z]{1,10}\.[a-z]{2,4}`).Draw(t, "binance.base_url_host")
	wsURL := "wss://" + rapid.StringMatching(`[a-z]{1,10}\.[a-z]{2,4}`).Draw(t, "binance.ws_url_host")

	// DatabaseConfig
	dbHost := rapid.StringMatching(`[a-z0-9]{1,15}`).Draw(t, "database.host")
	dbPort := rapid.IntRange(1, 65535).Draw(t, "database.port")
	dbUser := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "database.user")
	dbPassword := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "database.password")
	dbName := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "database.db_name")

	// LogConfig
	logLevel := rapid.SampledFrom([]string{"DEBUG", "INFO", "WARN", "ERROR"}).Draw(t, "log.level")
	logFilePath := rapid.StringMatching(`[a-z]{1,5}/[a-z]{1,8}\.log`).Draw(t, "log.file_path")
	maxSizeMB := rapid.IntRange(1, 1000).Draw(t, "log.max_size_mb")
	maxAgeDays := rapid.IntRange(1, 365).Draw(t, "log.max_age_days")

	// TradingConfig - 至少一个非空交易对
	numPairs := rapid.IntRange(1, 5).Draw(t, "trading.num_pairs")
	pairs := make([]string, numPairs)
	for i := 0; i < numPairs; i++ {
		pairs[i] = rapid.StringMatching(`[A-Z]{3,6}USDT`).Draw(t, fmt.Sprintf("trading.pair_%d", i))
	}

	// RiskConfig - 正数 decimal
	maxOrderAmt := decimal.NewFromInt(int64(rapid.IntRange(1, 100000).Draw(t, "risk.max_order_amount")))
	maxDailyLoss := decimal.NewFromInt(int64(rapid.IntRange(1, 50000).Draw(t, "risk.max_daily_loss")))

	// StopLossPercent 和 MaxPositionPercent 映射
	// 注意：Viper 会将所有 map 键转为小写，因此使用小写符号以确保往返一致性。
	numSymbols := rapid.IntRange(1, 3).Draw(t, "risk.num_symbols")
	stopLoss := make(map[string]decimal.Decimal, numSymbols)
	maxPos := make(map[string]decimal.Decimal, numSymbols)
	for i := 0; i < numSymbols; i++ {
		sym := rapid.StringMatching(`[a-z]{3,5}usdt`).Draw(t, fmt.Sprintf("risk.symbol_%d", i))
		slPct := rapid.IntRange(1, 50).Draw(t, fmt.Sprintf("risk.stop_loss_%d", i))
		stopLoss[sym] = decimal.NewFromFloat(float64(slPct) / 100.0)
		mpPct := rapid.IntRange(1, 100).Draw(t, fmt.Sprintf("risk.max_pos_%d", i))
		maxPos[sym] = decimal.NewFromFloat(float64(mpPct) / 100.0)
	}

	// OptimizerConfig
	intervalHours := rapid.IntRange(1, 168).Draw(t, "optimizer.interval_hours")
	lookbackDays := rapid.IntRange(1, 365).Draw(t, "optimizer.lookback_days")
	maxParamChange := float64(rapid.IntRange(1, 100).Draw(t, "optimizer.max_param_change_pct")) / 100.0

	return &Config{
		Server: ServerConfig{Host: host, Port: port, Mode: mode},
		Binance: BinanceConfig{
			APIKey: apiKey, SecretKey: secretKey,
			BaseURL: baseURL, WsURL: wsURL,
		},
		Database: DatabaseConfig{
			Host: dbHost, Port: dbPort, User: dbUser,
			Password: dbPassword, DBName: dbName,
		},
		Log: LogConfig{
			Level: logLevel, FilePath: logFilePath,
			MaxSizeMB: maxSizeMB, MaxAgeDays: maxAgeDays,
		},
		Trading: TradingConfig{DefaultPairs: pairs},
		Risk: RiskConfig{
			MaxOrderAmount: maxOrderAmt, MaxDailyLoss: maxDailyLoss,
			StopLossPercent: stopLoss, MaxPositionPercent: maxPos,
		},
		Optimizer: OptimizerConfig{
			Interval:       time.Duration(intervalHours) * time.Hour,
			LookbackDays:   lookbackDays,
			MaxParamChange: maxParamChange,
		},
	}
}

// configToYAML 将 Config 序列化为与 Viper 加载兼容的 YAML 字符串。
func configToYAML(cfg *Config) string {
	var b strings.Builder

	// Server
	b.WriteString("server:\n")
	b.WriteString(fmt.Sprintf("  host: %q\n", cfg.Server.Host))
	b.WriteString(fmt.Sprintf("  port: %d\n", cfg.Server.Port))
	b.WriteString(fmt.Sprintf("  mode: %q\n", cfg.Server.Mode))

	// Binance
	b.WriteString("binance:\n")
	b.WriteString(fmt.Sprintf("  api_key: %q\n", cfg.Binance.APIKey))
	b.WriteString(fmt.Sprintf("  secret_key: %q\n", cfg.Binance.SecretKey))
	b.WriteString(fmt.Sprintf("  base_url: %q\n", cfg.Binance.BaseURL))
	b.WriteString(fmt.Sprintf("  ws_url: %q\n", cfg.Binance.WsURL))

	// Database
	b.WriteString("database:\n")
	b.WriteString(fmt.Sprintf("  host: %q\n", cfg.Database.Host))
	b.WriteString(fmt.Sprintf("  port: %d\n", cfg.Database.Port))
	b.WriteString(fmt.Sprintf("  user: %q\n", cfg.Database.User))
	b.WriteString(fmt.Sprintf("  password: %q\n", cfg.Database.Password))
	b.WriteString(fmt.Sprintf("  db_name: %q\n", cfg.Database.DBName))

	// Log
	b.WriteString("log:\n")
	b.WriteString(fmt.Sprintf("  level: %q\n", cfg.Log.Level))
	b.WriteString(fmt.Sprintf("  file_path: %q\n", cfg.Log.FilePath))
	b.WriteString(fmt.Sprintf("  max_size_mb: %d\n", cfg.Log.MaxSizeMB))
	b.WriteString(fmt.Sprintf("  max_age_days: %d\n", cfg.Log.MaxAgeDays))

	// Trading
	b.WriteString("trading:\n")
	b.WriteString("  default_pairs:\n")
	for _, p := range cfg.Trading.DefaultPairs {
		b.WriteString(fmt.Sprintf("    - %q\n", p))
	}

	// Risk
	b.WriteString("risk:\n")
	b.WriteString(fmt.Sprintf("  max_order_amount: %q\n", cfg.Risk.MaxOrderAmount.String()))
	b.WriteString(fmt.Sprintf("  max_daily_loss: %q\n", cfg.Risk.MaxDailyLoss.String()))
	if len(cfg.Risk.StopLossPercent) > 0 {
		b.WriteString("  stop_loss_percent:\n")
		for k, v := range cfg.Risk.StopLossPercent {
			b.WriteString(fmt.Sprintf("    %s: %q\n", k, v.String()))
		}
	}
	if len(cfg.Risk.MaxPositionPercent) > 0 {
		b.WriteString("  max_position_percent:\n")
		for k, v := range cfg.Risk.MaxPositionPercent {
			b.WriteString(fmt.Sprintf("    %s: %q\n", k, v.String()))
		}
	}

	// Optimizer
	b.WriteString("optimizer:\n")
	b.WriteString(fmt.Sprintf("  interval: %q\n", cfg.Optimizer.Interval.String()))
	b.WriteString(fmt.Sprintf("  lookback_days: %d\n", cfg.Optimizer.LookbackDays))
	b.WriteString(fmt.Sprintf("  max_param_change: %v\n", cfg.Optimizer.MaxParamChange))

	return b.String()
}

// assertConfigEqual 逐字段比较两个 Config 结构体，
// 对 Decimal 字段使用 decimal.Equal() 进行比较。
func assertConfigEqual(rt *rapid.T, original *Config, loaded *Config) {
	// Server
	if original.Server.Host != loaded.Server.Host {
		rt.Fatalf("Server.Host mismatch: %q vs %q", original.Server.Host, loaded.Server.Host)
	}
	if original.Server.Port != loaded.Server.Port {
		rt.Fatalf("Server.Port mismatch: %d vs %d", original.Server.Port, loaded.Server.Port)
	}
	if original.Server.Mode != loaded.Server.Mode {
		rt.Fatalf("Server.Mode mismatch: %q vs %q", original.Server.Mode, loaded.Server.Mode)
	}

	// Binance
	if original.Binance.APIKey != loaded.Binance.APIKey {
		rt.Fatalf("Binance.APIKey mismatch: %q vs %q", original.Binance.APIKey, loaded.Binance.APIKey)
	}
	if original.Binance.SecretKey != loaded.Binance.SecretKey {
		rt.Fatalf("Binance.SecretKey mismatch: %q vs %q", original.Binance.SecretKey, loaded.Binance.SecretKey)
	}
	if original.Binance.BaseURL != loaded.Binance.BaseURL {
		rt.Fatalf("Binance.BaseURL mismatch: %q vs %q", original.Binance.BaseURL, loaded.Binance.BaseURL)
	}
	if original.Binance.WsURL != loaded.Binance.WsURL {
		rt.Fatalf("Binance.WsURL mismatch: %q vs %q", original.Binance.WsURL, loaded.Binance.WsURL)
	}

	// Database
	if original.Database.Host != loaded.Database.Host {
		rt.Fatalf("Database.Host mismatch: %q vs %q", original.Database.Host, loaded.Database.Host)
	}
	if original.Database.Port != loaded.Database.Port {
		rt.Fatalf("Database.Port mismatch: %d vs %d", original.Database.Port, loaded.Database.Port)
	}
	if original.Database.User != loaded.Database.User {
		rt.Fatalf("Database.User mismatch: %q vs %q", original.Database.User, loaded.Database.User)
	}
	if original.Database.Password != loaded.Database.Password {
		rt.Fatalf("Database.Password mismatch: %q vs %q", original.Database.Password, loaded.Database.Password)
	}
	if original.Database.DBName != loaded.Database.DBName {
		rt.Fatalf("Database.DBName mismatch: %q vs %q", original.Database.DBName, loaded.Database.DBName)
	}

	// Log
	if original.Log.Level != loaded.Log.Level {
		rt.Fatalf("Log.Level mismatch: %q vs %q", original.Log.Level, loaded.Log.Level)
	}
	if original.Log.FilePath != loaded.Log.FilePath {
		rt.Fatalf("Log.FilePath mismatch: %q vs %q", original.Log.FilePath, loaded.Log.FilePath)
	}
	if original.Log.MaxSizeMB != loaded.Log.MaxSizeMB {
		rt.Fatalf("Log.MaxSizeMB mismatch: %d vs %d", original.Log.MaxSizeMB, loaded.Log.MaxSizeMB)
	}
	if original.Log.MaxAgeDays != loaded.Log.MaxAgeDays {
		rt.Fatalf("Log.MaxAgeDays mismatch: %d vs %d", original.Log.MaxAgeDays, loaded.Log.MaxAgeDays)
	}

	// Trading
	if len(original.Trading.DefaultPairs) != len(loaded.Trading.DefaultPairs) {
		rt.Fatalf("Trading.DefaultPairs length mismatch: %d vs %d",
			len(original.Trading.DefaultPairs), len(loaded.Trading.DefaultPairs))
	}
	for i, p := range original.Trading.DefaultPairs {
		if p != loaded.Trading.DefaultPairs[i] {
			rt.Fatalf("Trading.DefaultPairs[%d] mismatch: %q vs %q", i, p, loaded.Trading.DefaultPairs[i])
		}
	}

	// Risk - 对 Decimal 字段使用 decimal.Equal
	if !original.Risk.MaxOrderAmount.Equal(loaded.Risk.MaxOrderAmount) {
		rt.Fatalf("Risk.MaxOrderAmount mismatch: %s vs %s",
			original.Risk.MaxOrderAmount, loaded.Risk.MaxOrderAmount)
	}
	if !original.Risk.MaxDailyLoss.Equal(loaded.Risk.MaxDailyLoss) {
		rt.Fatalf("Risk.MaxDailyLoss mismatch: %s vs %s",
			original.Risk.MaxDailyLoss, loaded.Risk.MaxDailyLoss)
	}
	for k, v := range original.Risk.StopLossPercent {
		lv, ok := loaded.Risk.StopLossPercent[k]
		if !ok {
			rt.Fatalf("Risk.StopLossPercent missing key %q", k)
		}
		if !v.Equal(lv) {
			rt.Fatalf("Risk.StopLossPercent[%s] mismatch: %s vs %s", k, v, lv)
		}
	}
	for k, v := range original.Risk.MaxPositionPercent {
		lv, ok := loaded.Risk.MaxPositionPercent[k]
		if !ok {
			rt.Fatalf("Risk.MaxPositionPercent missing key %q", k)
		}
		if !v.Equal(lv) {
			rt.Fatalf("Risk.MaxPositionPercent[%s] mismatch: %s vs %s", k, v, lv)
		}
	}

	// Optimizer
	if original.Optimizer.Interval != loaded.Optimizer.Interval {
		rt.Fatalf("Optimizer.Interval mismatch: %v vs %v",
			original.Optimizer.Interval, loaded.Optimizer.Interval)
	}
	if original.Optimizer.LookbackDays != loaded.Optimizer.LookbackDays {
		rt.Fatalf("Optimizer.LookbackDays mismatch: %d vs %d",
			original.Optimizer.LookbackDays, loaded.Optimizer.LookbackDays)
	}
	if original.Optimizer.MaxParamChange != loaded.Optimizer.MaxParamChange {
		rt.Fatalf("Optimizer.MaxParamChange mismatch: %v vs %v",
			original.Optimizer.MaxParamChange, loaded.Optimizer.MaxParamChange)
	}
}

// Feature: binance-trading-system, Property 26: 配置验证拒绝无效配置
// **Validates: Requirements 12.6**
// Property 26: 对于任意缺少必填字段或类型错误的配置，验证器应返回错误并拒绝加载
func TestProperty26_ValidateRejectsInvalidConfig(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 从一个有效配置开始
		cfg := drawValidConfig(t)

		// 健全性检查：有效配置必须通过验证
		if err := cfg.Validate(); err != nil {
			t.Fatalf("drawValidConfig produced invalid config: %v", err)
		}

		// 随机选择一种破坏策略进行应用
		corruptionID := rapid.IntRange(0, 12).Draw(t, "corruption_id")

		switch corruptionID {
		case 0:
			// 将必填的 Binance 字符串字段设为空
			field := rapid.IntRange(0, 3).Draw(t, "binance_field")
			switch field {
			case 0:
				cfg.Binance.APIKey = ""
			case 1:
				cfg.Binance.SecretKey = ""
			case 2:
				cfg.Binance.BaseURL = ""
			case 3:
				cfg.Binance.WsURL = ""
			}

		case 1:
			// 将服务器端口设为无效值：0、负数或 > 65535
			choice := rapid.IntRange(0, 2).Draw(t, "port_choice")
			switch choice {
			case 0:
				cfg.Server.Port = 0
			case 1:
				cfg.Server.Port = -rapid.IntRange(1, 10000).Draw(t, "neg_port")
			case 2:
				cfg.Server.Port = 65536 + rapid.IntRange(0, 10000).Draw(t, "high_port")
			}

		case 2:
			// 将数据库端口设为无效值
			choice := rapid.IntRange(0, 2).Draw(t, "db_port_choice")
			switch choice {
			case 0:
				cfg.Database.Port = 0
			case 1:
				cfg.Database.Port = -rapid.IntRange(1, 10000).Draw(t, "neg_db_port")
			case 2:
				cfg.Database.Port = 65536 + rapid.IntRange(0, 10000).Draw(t, "high_db_port")
			}

		case 3:
			// 将必填的数据库字符串字段设为空
			field := rapid.IntRange(0, 3).Draw(t, "db_field")
			switch field {
			case 0:
				cfg.Database.Host = ""
			case 1:
				cfg.Database.User = ""
			case 2:
				cfg.Database.Password = ""
			case 3:
				cfg.Database.DBName = ""
			}

		case 4:
			// 将日志级别设为无效字符串
			cfg.Log.Level = rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "bad_log_level")

		case 5:
			// 将 log.max_size_mb 设为零或负数
			cfg.Log.MaxSizeMB = -rapid.IntRange(0, 1000).Draw(t, "bad_max_size")

		case 6:
			// 将 log.max_age_days 设为零或负数
			cfg.Log.MaxAgeDays = -rapid.IntRange(0, 1000).Draw(t, "bad_max_age")

		case 7:
			// 将 default_pairs 设为空切片
			cfg.Trading.DefaultPairs = []string{}

		case 8:
			// 将 risk.max_order_amount 设为零
			cfg.Risk.MaxOrderAmount = decimal.Zero

		case 9:
			// 将 risk.max_order_amount 设为负数
			cfg.Risk.MaxOrderAmount = decimal.NewFromInt(-int64(rapid.IntRange(1, 100000).Draw(t, "neg_order_amt")))

		case 10:
			// 将 risk.max_daily_loss 设为零或负数
			choice := rapid.IntRange(0, 1).Draw(t, "daily_loss_choice")
			if choice == 0 {
				cfg.Risk.MaxDailyLoss = decimal.Zero
			} else {
				cfg.Risk.MaxDailyLoss = decimal.NewFromInt(-int64(rapid.IntRange(1, 50000).Draw(t, "neg_daily_loss")))
			}

		case 11:
			// 将 optimizer.max_param_change 设为 <= 0 或 > 1.0
			choice := rapid.IntRange(0, 1).Draw(t, "param_change_choice")
			if choice == 0 {
				// <= 0
				cfg.Optimizer.MaxParamChange = -rapid.Float64Range(0, 10).Draw(t, "neg_param_change")
			} else {
				// > 1.0
				cfg.Optimizer.MaxParamChange = 1.0 + rapid.Float64Range(0.001, 10).Draw(t, "high_param_change")
			}

		case 12:
			// 将 optimizer.lookback_days 或 interval 设为零/负数
			field := rapid.IntRange(0, 1).Draw(t, "opt_field")
			if field == 0 {
				cfg.Optimizer.LookbackDays = -rapid.IntRange(0, 365).Draw(t, "neg_lookback")
			} else {
				cfg.Optimizer.Interval = 0
			}
		}

		// 被破坏的配置必须验证失败
		err := cfg.Validate()
		if err == nil {
			t.Fatalf("expected Validate() to return error for corruption %d, but got nil", corruptionID)
		}
	})
}
