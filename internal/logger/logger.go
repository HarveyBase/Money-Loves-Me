package logger

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"money-loves-me/internal/config"
)

// Logger 封装 zap.Logger，并在每条日志中自动包含模块字段。
type Logger struct {
	zap    *zap.Logger
	module string
}

// NewLogger 为给定模块创建一个新的结构化日志记录器。
// 它将 JSON 格式的日志同时写入标准输出和轮转日志文件。
func NewLogger(module string, cfg config.LogConfig) (*Logger, error) {
	if module == "" {
		return nil, fmt.Errorf("module name must not be empty")
	}

	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "module",
		MessageKey:     "message",
		CallerKey:      "",
		StacktraceKey:  "",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	jsonEncoder := zapcore.NewJSONEncoder(encoderCfg)

	// 控制台输出（标准输出）
	consoleSyncer := zapcore.AddSync(os.Stdout)

	var cores []zapcore.Core
	cores = append(cores, zapcore.NewCore(jsonEncoder, consoleSyncer, level))

	// 文件输出与轮转（仅在设置了 FilePath 时启用）
	if cfg.FilePath != "" {
		maxSize := cfg.MaxSizeMB
		if maxSize <= 0 {
			maxSize = 100 // 默认 100MB
		}
		maxAge := cfg.MaxAgeDays
		if maxAge <= 0 {
			maxAge = 30 // 默认 30 天
		}

		rotator := &lumberjack.Logger{
			Filename: cfg.FilePath,
			MaxSize:  maxSize, // 兆字节
			MaxAge:   maxAge,  // 天
			Compress: false,
		}
		fileSyncer := zapcore.AddSync(rotator)
		cores = append(cores, zapcore.NewCore(jsonEncoder, fileSyncer, level))
	}

	core := zapcore.NewTee(cores...)
	zapLogger := zap.New(core).Named(module)

	return &Logger{
		zap:    zapLogger,
		module: module,
	}, nil
}

// Debug 以 DEBUG 级别记录日志消息。
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.zap.Debug(msg, fields...)
}

// Info 以 INFO 级别记录日志消息。
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.zap.Info(msg, fields...)
}

// Warn 以 WARN 级别记录日志消息。
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.zap.Warn(msg, fields...)
}

// Error 以 ERROR 级别记录日志消息。
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.zap.Error(msg, fields...)
}

// Sync 刷新所有缓冲的日志条目。
func (l *Logger) Sync() error {
	return l.zap.Sync()
}

// Module 返回与此日志记录器关联的模块名称。
func (l *Logger) Module() string {
	return l.module
}

// Zap 返回底层的 zap.Logger，用于高级用法。
func (l *Logger) Zap() *zap.Logger {
	return l.zap
}

// parseLevel 将字符串日志级别转换为 zapcore.Level。
func parseLevel(level string) (zapcore.Level, error) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return zapcore.DebugLevel, nil
	case "INFO":
		return zapcore.InfoLevel, nil
	case "WARN":
		return zapcore.WarnLevel, nil
	case "ERROR":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("invalid log level %q: must be DEBUG, INFO, WARN, or ERROR", level)
	}
}
