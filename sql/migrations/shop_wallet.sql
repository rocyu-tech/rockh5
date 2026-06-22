-- Shop/Wallet Migration for RockGame
-- Run this against the rockgame database

-- 1. payment_channel (non-sharded) — payment method configurations
CREATE TABLE IF NOT EXISTS `payment_channel` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `name` VARCHAR(64) NOT NULL DEFAULT '',
  `type` VARCHAR(32) NOT NULL DEFAULT 'general' COMMENT 'usdt, bank, upi, crypto, stripe, paypal',
  `icon` VARCHAR(256) NOT NULL DEFAULT '',
  `sub_title` VARCHAR(128) NOT NULL DEFAULT '',
  `channel_type` TINYINT NOT NULL DEFAULT 2 COMMENT '0=charge, 1=withdraw, 2=both',
  `min_charge` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `max_charge` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `min_withdraw` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `max_withdraw` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `daily_limit` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `is_hot` TINYINT(1) NOT NULL DEFAULT 0,
  `sort_order` INT NOT NULL DEFAULT 0,
  `status` TINYINT NOT NULL DEFAULT 1,
  `config_json` TEXT NOT NULL DEFAULT '{}',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX `idx_status` (`status`),
  INDEX `idx_channel_type` (`channel_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Insert default channels
INSERT IGNORE INTO `payment_channel` (`name`, `type`, `icon`, `channel_type`, `min_charge`, `max_charge`, `min_withdraw`, `max_withdraw`, `daily_limit`, `sort_order`, `status`) VALUES
('USDT-TRC20', 'usdt', '', 2, 10, 50000, 10, 50000, 100000, 1, 1),
('Bank Transfer', 'bank', '', 2, 20, 10000, 20, 10000, 50000, 2, 1),
('UPI', 'upi', '', 2, 5, 5000, 5, 5000, 20000, 3, 1),
('Stripe', 'stripe', '', 0, 5, 10000, 0, 0, 0, 4, 0);

-- 2. user_wallet (non-sharded, one row per user)
CREATE TABLE IF NOT EXISTS `user_wallet` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `user_id` BIGINT NOT NULL UNIQUE,
  `cash_balance` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `bonus_balance` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `frozen_balance` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `total_recharge` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `total_withdraw` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `recharge_count` INT NOT NULL DEFAULT 0,
  `withdraw_count` INT NOT NULL DEFAULT 0,
  `total_bet` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `total_win` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `flow_required` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `flow_completed` DECIMAL(18,4) NOT NULL DEFAULT 0,
  `version` INT NOT NULL DEFAULT 0 COMMENT 'optimistic lock',
  `withdraw_pwd_hash` VARCHAR(128) NOT NULL DEFAULT '',
  `withdraw_pwd_set` TINYINT(1) NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 3. recharge_order_{00..15} (sharded by user_id % 16)
-- Uses same schema as existing — check if table already exists before creating
-- The sharded tables are created by the existing schema.sql, ensure they have these columns:

-- ALTER TABLE recharge_order ADD COLUMN IF NOT EXISTS:
-- (These ALTER statements are idempotent-safe for reference)

-- 4. withdraw_order_{00..15} (sharded by user_id % 16)
-- Same sharding approach

-- 5. user_payment_account_{00..15} (sharded by user_id % 16)
CREATE TABLE IF NOT EXISTS `user_payment_account_00` (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `user_id` BIGINT NOT NULL,
  `account_type` INT NOT NULL DEFAULT 0 COMMENT '0=bank, 1=usdt, 2=upi',
  `title` VARCHAR(64) NOT NULL DEFAULT '',
  `account` VARCHAR(256) NOT NULL,
  `code` VARCHAR(64) NOT NULL DEFAULT '',
  `username` VARCHAR(128) NOT NULL DEFAULT '',
  `modify_count` INT NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX `idx_user_id` (`user_id`),
  INDEX `idx_account` (`account`(64))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Create remaining 15 shards (01..15) using same schema
-- Run this for i in 01..15:
-- CREATE TABLE IF NOT EXISTS `user_payment_account_XX` LIKE `user_payment_account_00`;
