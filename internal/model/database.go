package model

import (
	"fmt"

	"money-loves-me/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB 根据配置初始化数据库连接，支持 MySQL 和 SQLite。
// 当 database.driver 为 "sqlite" 时使用 SQLite，否则默认使用 MySQL。
func InitDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	if cfg.Driver == "sqlite" {
		return initSQLite(cfg)
	}
	return initMySQL(cfg)
}

// initMySQL 使用提供的 DatabaseConfig 初始化 MySQL 数据库连接。
func initMySQL(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)

	return db, nil
}

// initSQLite 使用 SQLite 文件初始化数据库连接，适合本地开发。
func initSQLite(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dbPath := cfg.DBName
	if dbPath == "" {
		dbPath = "data/trading.db"
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	return db, nil
}

// AutoMigrate 对所有已注册的模型执行 GORM 自动迁移。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Strategy{},
		&Order{},
		&Trade{},
		&AccountSnapshot{},
		&BacktestResult{},
		&OptimizationRecord{},
		&Notification{},
		&RiskConfig{},
		&NotificationSetting{},
	)
}
