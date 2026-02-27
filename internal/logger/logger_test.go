package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"money-loves-me/internal/config"

	"pgregory.net/rapid"
)

func TestNewLogger_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := config.LogConfig{
		Level:      "INFO",
		FilePath:   logFile,
		MaxSizeMB:  100,
		MaxAgeDays: 30,
	}

	l, err := NewLogger("test-module", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer l.Sync()

	if l.Module() != "test-module" {
		t.Errorf("expected module %q, got %q", "test-module", l.Module())
	}
}

func TestNewLogger_EmptyModule(t *testing.T) {
	cfg := config.LogConfig{Level: "INFO", MaxSizeMB: 100, MaxAgeDays: 30}
	_, err := NewLogger("", cfg)
	if err == nil {
		t.Fatal("expected error for empty module name")
	}
}

func TestNewLogger_InvalidLevel(t *testing.T) {
	cfg := config.LogConfig{Level: "TRACE", MaxSizeMB: 100, MaxAgeDays: 30}
	_, err := NewLogger("mod", cfg)
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestNewLogger_AllLevels(t *testing.T) {
	for _, level := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
		t.Run(level, func(t *testing.T) {
			cfg := config.LogConfig{Level: level, MaxSizeMB: 100, MaxAgeDays: 30}
			l, err := NewLogger("mod", cfg)
			if err != nil {
				t.Fatalf("unexpected error for level %s: %v", level, err)
			}
			l.Sync()
		})
	}
}

func TestLogger_WritesToFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := config.LogConfig{
		Level:      "DEBUG",
		FilePath:   logFile,
		MaxSizeMB:  100,
		MaxAgeDays: 30,
	}

	l, err := NewLogger("file-test", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	l.Info("hello world")
	l.Sync()

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("log entry is not valid JSON: %v\nraw: %s", err, string(data))
	}

	// 验证所有必需字段是否存在
	if _, ok := entry["timestamp"]; !ok {
		t.Error("missing 'timestamp' field")
	}
	if _, ok := entry["level"]; !ok {
		t.Error("missing 'level' field")
	}
	if _, ok := entry["module"]; !ok {
		t.Error("missing 'module' field")
	}
	if _, ok := entry["message"]; !ok {
		t.Error("missing 'message' field")
	}

	if entry["module"] != "file-test" {
		t.Errorf("expected module 'file-test', got %v", entry["module"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("expected level 'INFO', got %v", entry["level"])
	}
	if entry["message"] != "hello world" {
		t.Errorf("expected message 'hello world', got %v", entry["message"])
	}
}

func TestLogger_AllLogMethods(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "methods.log")

	cfg := config.LogConfig{
		Level:      "DEBUG",
		FilePath:   logFile,
		MaxSizeMB:  100,
		MaxAgeDays: 30,
	}

	l, err := NewLogger("methods-test", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	l.Debug("debug msg")
	l.Info("info msg")
	l.Warn("warn msg")
	l.Error("error msg")
	l.Sync()

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// 文件应包含多行 JSON
	lines := splitJSONLines(data)
	if len(lines) != 4 {
		t.Fatalf("expected 4 log entries, got %d", len(lines))
	}

	expectedLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	expectedMsgs := []string{"debug msg", "info msg", "warn msg", "error msg"}

	for i, line := range lines {
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", i, err)
		}
		if entry["level"] != expectedLevels[i] {
			t.Errorf("line %d: expected level %q, got %v", i, expectedLevels[i], entry["level"])
		}
		if entry["message"] != expectedMsgs[i] {
			t.Errorf("line %d: expected message %q, got %v", i, expectedMsgs[i], entry["message"])
		}
		if entry["module"] != "methods-test" {
			t.Errorf("line %d: expected module 'methods-test', got %v", i, entry["module"])
		}
		if _, ok := entry["timestamp"]; !ok {
			t.Errorf("line %d: missing timestamp", i)
		}
	}
}

func TestLogger_DefaultRotationValues(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "defaults.log")

	// 零值应使用默认值（100MB，30 天）
	cfg := config.LogConfig{
		Level:      "INFO",
		FilePath:   logFile,
		MaxSizeMB:  0,
		MaxAgeDays: 0,
	}

	l, err := NewLogger("defaults", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	l.Info("test default rotation")
	l.Sync()

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("log file was not created")
	}
}

func TestLogger_NoFilePath(t *testing.T) {
	// 未设置文件路径时，日志记录器应仍能工作（仅标准输出）
	cfg := config.LogConfig{
		Level:      "INFO",
		MaxSizeMB:  100,
		MaxAgeDays: 30,
	}

	l, err := NewLogger("stdout-only", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 不应 panic
	l.Info("stdout only message")
	l.Sync()
}

func TestLogger_Zap(t *testing.T) {
	cfg := config.LogConfig{Level: "INFO", MaxSizeMB: 100, MaxAgeDays: 30}
	l, err := NewLogger("zap-test", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l.Zap() == nil {
		t.Error("Zap() returned nil")
	}
}

// splitJSONLines 将原始字节分割为单独的 JSON 行，跳过空行。
func splitJSONLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			line := data[start:i]
			if len(line) > 0 {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(data) {
		line := data[start:]
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}

// Feature: binance-trading-system, Property 30: 结构化日志完整性
// Validates: Requirements 9.3
// Property 30: 对于任意日志条目，必须包含时间戳、模块名称、日志级别和日志内容，且均不为空
func TestProperty30_StructuredLogCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// 生成一个随机的非空字母数字模块名称
		module := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{0,29}`).Draw(t, "module")

		// 生成一个随机的非空日志消息
		message := rapid.StringMatching(`[a-zA-Z0-9 ]{1,100}`).Draw(t, "message")

		// 随机选择一个日志级别
		level := rapid.SampledFrom([]string{"DEBUG", "INFO", "WARN", "ERROR"}).Draw(t, "level")

		// 创建临时目录用于日志输出
		tmpDir, err := os.MkdirTemp("", "prop30-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)
		logFile := filepath.Join(tmpDir, "prop30.log")

		cfg := config.LogConfig{
			Level:      "DEBUG", // 使用 DEBUG 以便捕获所有级别的日志
			FilePath:   logFile,
			MaxSizeMB:  100,
			MaxAgeDays: 30,
		}

		l, err := NewLogger(module, cfg)
		if err != nil {
			t.Fatalf("failed to create logger: %v", err)
		}

		// 以选定的级别写入日志条目
		switch level {
		case "DEBUG":
			l.Debug(message)
		case "INFO":
			l.Info(message)
		case "WARN":
			l.Warn(message)
		case "ERROR":
			l.Error(message)
		}
		l.Sync()

		// 读取并解析日志文件
		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}

		lines := splitJSONLines(data)
		if len(lines) == 0 {
			t.Fatal("no log entries found in file")
		}

		// 解析最后一行（我们的日志条目）
		var entry map[string]any
		if err := json.Unmarshal(lines[len(lines)-1], &entry); err != nil {
			t.Fatalf("log entry is not valid JSON: %v", err)
		}

		// 断言所有 4 个字段存在且非空
		timestamp, ok := entry["timestamp"].(string)
		if !ok || timestamp == "" {
			t.Fatalf("timestamp field missing or empty: %v", entry["timestamp"])
		}

		levelVal, ok := entry["level"].(string)
		if !ok || levelVal == "" {
			t.Fatalf("level field missing or empty: %v", entry["level"])
		}

		moduleVal, ok := entry["module"].(string)
		if !ok || moduleVal == "" {
			t.Fatalf("module field missing or empty: %v", entry["module"])
		}

		messageVal, ok := entry["message"].(string)
		if !ok || messageVal == "" {
			t.Fatalf("message field missing or empty: %v", entry["message"])
		}

		// 验证值与写入的内容匹配
		if levelVal != level {
			t.Fatalf("expected level %q, got %q", level, levelVal)
		}
		if moduleVal != module {
			t.Fatalf("expected module %q, got %q", module, moduleVal)
		}
		if messageVal != message {
			t.Fatalf("expected message %q, got %q", message, messageVal)
		}
	})
}
