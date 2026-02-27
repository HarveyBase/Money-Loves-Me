-- Migration: 001_init.sql
-- Description: Initial schema for Binance Trading System (money-loves-me)
-- Creates all core tables with indexes, foreign keys, and constraints.

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- -----------------------------------------------------------
-- 1. users
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `users` (
    `id`                 INT            NOT NULL AUTO_INCREMENT,
    `username`           VARCHAR(64)    NOT NULL,
    `password_hash`      VARCHAR(255)   NOT NULL,
    `failed_login_count` INT            NOT NULL DEFAULT 0,
    `locked_until`       TIMESTAMP      NULL DEFAULT NULL,
    `created_at`         TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_users_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------
-- 2. strategies
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `strategies` (
    `id`         INT            NOT NULL AUTO_INCREMENT,
    `name`       VARCHAR(50)    NOT NULL,
    `type`       VARCHAR(50)    NOT NULL,
    `params`     JSON           NOT NULL,
    `active`     TINYINT(1)     NOT NULL DEFAULT 0,
    `updated_at` TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_strategies_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------
-- 3. orders
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `orders` (
    `id`               BIGINT         NOT NULL AUTO_INCREMENT,
    `symbol`           VARCHAR(20)    NOT NULL,
    `side`             VARCHAR(4)     NOT NULL COMMENT 'BUY/SELL',
    `type`             VARCHAR(30)    NOT NULL COMMENT 'LIMIT/MARKET/STOP_LOSS_LIMIT/TAKE_PROFIT_LIMIT',
    `quantity`         DECIMAL(20,8)  NOT NULL,
    `price`            DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `stop_price`       DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `status`           VARCHAR(20)    NOT NULL DEFAULT 'SUBMITTED' COMMENT 'SUBMITTED/PARTIAL/FILLED/CANCELLED',
    `binance_order_id` BIGINT         NULL DEFAULT NULL,
    `fee`              DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `fee_asset`        VARCHAR(10)    NOT NULL DEFAULT '',
    `strategy_name`    VARCHAR(50)    NOT NULL DEFAULT '',
    `created_at`       TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`       TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_orders_symbol` (`symbol`),
    INDEX `idx_orders_status` (`status`),
    INDEX `idx_orders_strategy_name` (`strategy_name`),
    INDEX `idx_orders_created_at` (`created_at`),
    INDEX `idx_orders_binance_order_id` (`binance_order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------
-- 4. trades
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `trades` (
    `id`              BIGINT         NOT NULL AUTO_INCREMENT,
    `order_id`        BIGINT         NOT NULL,
    `symbol`          VARCHAR(20)    NOT NULL,
    `side`            VARCHAR(4)     NOT NULL COMMENT 'BUY/SELL',
    `price`           DECIMAL(20,8)  NOT NULL,
    `quantity`        DECIMAL(20,8)  NOT NULL,
    `amount`          DECIMAL(20,8)  NOT NULL,
    `fee`             DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `fee_asset`       VARCHAR(10)    NOT NULL DEFAULT '',
    `strategy_name`   VARCHAR(50)    NOT NULL DEFAULT '',
    `decision_reason` JSON           NULL,
    `balance_before`  DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `balance_after`   DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `executed_at`     TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_trades_order_id` (`order_id`),
    INDEX `idx_trades_symbol` (`symbol`),
    INDEX `idx_trades_strategy_name` (`strategy_name`),
    INDEX `idx_trades_executed_at` (`executed_at`),
    CONSTRAINT `fk_trades_order_id` FOREIGN KEY (`order_id`) REFERENCES `orders` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------
-- 5. account_snapshots
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `account_snapshots` (
    `id`               BIGINT         NOT NULL AUTO_INCREMENT,
    `total_value_usdt` DECIMAL(20,8)  NOT NULL,
    `balances`         JSON           NOT NULL,
    `snapshot_at`      TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_account_snapshots_snapshot_at` (`snapshot_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------
-- 6. backtest_results
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `backtest_results` (
    `id`              BIGINT         NOT NULL AUTO_INCREMENT,
    `strategy_name`   VARCHAR(50)    NOT NULL,
    `symbol`          VARCHAR(20)    NOT NULL,
    `params`          JSON           NOT NULL,
    `start_time`      TIMESTAMP      NOT NULL,
    `end_time`        TIMESTAMP      NOT NULL,
    `initial_capital` DECIMAL(20,8)  NOT NULL,
    `total_return`    DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `net_profit`      DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `max_drawdown`    DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `win_rate`        DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `profit_factor`   DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `total_trades`    INT            NOT NULL DEFAULT 0,
    `total_fees`      DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `equity_curve`    JSON           NULL,
    `trades`          JSON           NULL,
    `slippage`        DECIMAL(20,8)  NOT NULL DEFAULT 0.00000000,
    `created_at`      TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_backtest_results_strategy_name` (`strategy_name`),
    INDEX `idx_backtest_results_symbol` (`symbol`),
    INDEX `idx_backtest_results_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------
-- 7. optimization_records
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `optimization_records` (
    `id`              BIGINT         NOT NULL AUTO_INCREMENT,
    `strategy_name`   VARCHAR(50)    NOT NULL,
    `old_params`      JSON           NOT NULL,
    `new_params`      JSON           NOT NULL,
    `old_metrics`     JSON           NOT NULL,
    `new_metrics`     JSON           NOT NULL,
    `analysis_notes`  TEXT           NULL,
    `applied`         TINYINT(1)     NOT NULL DEFAULT 0,
    `created_at`      TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_optimization_records_strategy_name` (`strategy_name`),
    INDEX `idx_optimization_records_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------
-- 8. notifications
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `notifications` (
    `id`          BIGINT         NOT NULL AUTO_INCREMENT,
    `event_type`  VARCHAR(30)    NOT NULL,
    `title`       VARCHAR(255)   NOT NULL,
    `description` TEXT           NULL,
    `is_read`     TINYINT(1)     NOT NULL DEFAULT 0,
    `created_at`  TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_notifications_event_type` (`event_type`),
    INDEX `idx_notifications_is_read` (`is_read`),
    INDEX `idx_notifications_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------
-- 9. risk_configs
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `risk_configs` (
    `id`                    INT            NOT NULL AUTO_INCREMENT,
    `max_order_amount`      DECIMAL(20,8)  NOT NULL,
    `max_daily_loss`        DECIMAL(20,8)  NOT NULL,
    `stop_loss_percents`    JSON           NOT NULL,
    `max_position_percents` JSON           NOT NULL,
    `updated_at`            TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------
-- 10. notification_settings
-- -----------------------------------------------------------
CREATE TABLE IF NOT EXISTS `notification_settings` (
    `id`             INT            NOT NULL AUTO_INCREMENT,
    `user_id`        INT            NOT NULL,
    `enabled_events` JSON           NOT NULL,
    `updated_at`     TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_notification_settings_user_id` (`user_id`),
    CONSTRAINT `fk_notification_settings_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET FOREIGN_KEY_CHECKS = 1;
