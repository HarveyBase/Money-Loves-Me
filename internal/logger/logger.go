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

// Logger wraps zap.Logger and automatically includes a module field in every log entry.
type Logger struct {
	zap    *zap.Logger
	module string
}

// NewLogger creates a new structured logger for the given module.
// It writes JSON-formatted logs to both stdout and a rotating log file.
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

	// Console output (stdout)
	consoleSyncer := zapcore.AddSync(os.Stdout)

	var cores []zapcore.Core
	cores = append(cores, zapcore.NewCore(jsonEncoder, consoleSyncer, level))

	// File output with rotation (only if FilePath is set)
	if cfg.FilePath != "" {
		maxSize := cfg.MaxSizeMB
		if maxSize <= 0 {
			maxSize = 100 // default 100MB
		}
		maxAge := cfg.MaxAgeDays
		if maxAge <= 0 {
			maxAge = 30 // default 30 days
		}

		rotator := &lumberjack.Logger{
			Filename: cfg.FilePath,
			MaxSize:  maxSize, // megabytes
			MaxAge:   maxAge,  // days
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

// Debug logs a message at DEBUG level.
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.zap.Debug(msg, fields...)
}

// Info logs a message at INFO level.
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.zap.Info(msg, fields...)
}

// Warn logs a message at WARN level.
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.zap.Warn(msg, fields...)
}

// Error logs a message at ERROR level.
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.zap.Error(msg, fields...)
}

// Sync flushes any buffered log entries.
func (l *Logger) Sync() error {
	return l.zap.Sync()
}

// Module returns the module name associated with this logger.
func (l *Logger) Module() string {
	return l.module
}

// Zap returns the underlying zap.Logger for advanced usage.
func (l *Logger) Zap() *zap.Logger {
	return l.zap
}

// parseLevel converts a string log level to a zapcore.Level.
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
