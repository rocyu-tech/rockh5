-- ============================================================================
-- Shop Module Migration: H5 Wallet Page Backend Support
-- Date: 2026-06-22
-- ============================================================================
-- This migration adds:
--   1. user_payment_account table (sharded) — user's saved payment methods
--   2. user_withdraw_password table (non-sharded) — withdraw password security
--   3. withdraw_order columns: channel_id, fee, real_amount
--   4. Seed data: 2 payment channels (Stripe, USDT)
-- ============================================================================

-- 1. user_payment_account (hash-sharded by user_id)
CREATE TABLE IF NOT EXISTS `user_payment_account_00` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT       NOT NULL COMMENT '用户ID',
    `account_type` TINYINT      NOT NULL COMMENT '1=bank 2=usdt 3=paypal',
    `title`        VARCHAR(64)  NOT NULL COMMENT 'display name',
    `account`      VARCHAR(256) NOT NULL COMMENT 'account number / address',
    `code`         VARCHAR(64)  NOT NULL DEFAULT '' COMMENT 'bank code / network',
    `username`     VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'account holder name',
    `is_default`   TINYINT      NOT NULL DEFAULT 0 COMMENT '1=default',
    `status`       TINYINT      NOT NULL DEFAULT 1 COMMENT '1=active 0=deleted',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user` (`user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户收款账户(分表)';

-- Create all 16 shards
CREATE TABLE IF NOT EXISTS `user_payment_account_01` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_02` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_03` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_04` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_05` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_06` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_07` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_08` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_09` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_10` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_11` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_12` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_13` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_14` LIKE `user_payment_account_00`;
CREATE TABLE IF NOT EXISTS `user_payment_account_15` LIKE `user_payment_account_00`;

-- 2. user_withdraw_password (non-sharded)
CREATE TABLE IF NOT EXISTS `user_withdraw_password` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT       NOT NULL COMMENT '用户ID',
    `password_hash` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'bcrypt hash',
    `has_set`       TINYINT      NOT NULL DEFAULT 0 COMMENT '0=未设置 1=已设置',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户提现密码';

-- 3. Add missing columns to withdraw_order (if not exists)
-- These are needed for the H5 wallet withdraw flow
ALTER TABLE `withdraw_order` ADD COLUMN IF NOT EXISTS `channel_id` BIGINT NOT NULL DEFAULT 0 COMMENT '支付通道ID' AFTER `bank_info`;
ALTER TABLE `withdraw_order` ADD COLUMN IF NOT EXISTS `fee` DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '手续费' AFTER `channel_id`;
ALTER TABLE `withdraw_order` ADD COLUMN IF NOT EXISTS `real_amount` DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '实际到账金额' AFTER `fee`;

-- Apply same columns to all sharded withdraw_order tables
-- (Only if they use sharding; the base schema already has the columns in schema.sql,
--  so this is a safety net for existing deployments)

-- 4. Seed payment channels for development/testing
INSERT IGNORE INTO `payment_channel` (`id`, `name`, `type`, `config_json`, `status`, `supported_regions`, `min_amount`, `max_amount`, `sort_order`) VALUES
(1, 'Credit Card', 'stripe', '{}', 1, '', 10.0000, 10000.0000, 1),
(2, 'USDT (TRC20)', 'usdt', '{}', 1, '', 50.0000, 50000.0000, 2),
(3, 'Bank Transfer', 'bank', '{}', 1, '', 20.0000, 50000.0000, 3);

-- 5. Create sharded recharge_order and withdraw_order tables if they don't exist
-- (The base schema.sql only defines the unsharded versions; mesh services use sharded)
CREATE TABLE IF NOT EXISTS `recharge_order_00` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT       NOT NULL COMMENT '用户ID',
    `order_no`      VARCHAR(64)  NOT NULL COMMENT '平台订单号',
    `amount`        DECIMAL(18,4) NOT NULL COMMENT '充值金额(原始币种)',
    `amount_usd`    DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '换算后USD金额',
    `currency`      VARCHAR(8)   NOT NULL DEFAULT 'USD' COMMENT '原始币种',
    `channel_id`    BIGINT       NOT NULL COMMENT '支付通道ID',
    `third_order_no` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '三方订单号',
    `status`        TINYINT      NOT NULL DEFAULT 0 COMMENT '0=待支付 1=已支付 2=失败 3=已退款',
    `paid_at`       DATETIME     NULL     COMMENT '支付完成时间',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_order_no` (`order_no`),
    KEY `idx_user_time` (`user_id`, `created_at`),
    KEY `idx_third_order` (`third_order_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='充值订单(分表)';

CREATE TABLE IF NOT EXISTS `withdraw_order_00` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT       NOT NULL COMMENT '用户ID',
    `order_no`     VARCHAR(64)  NOT NULL COMMENT '平台订单号',
    `amount`       DECIMAL(18,4) NOT NULL COMMENT '提现金额(USD)',
    `currency`     VARCHAR(8)   NOT NULL DEFAULT 'USD' COMMENT '提现币种',
    `bank_info`    JSON         NOT NULL COMMENT '收款信息(加密)',
    `channel_id`   BIGINT       NOT NULL DEFAULT 0 COMMENT '支付通道ID',
    `fee`          DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '手续费',
    `real_amount`  DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '实际到账金额',
    `status`       TINYINT      NOT NULL DEFAULT 0 COMMENT '0=待审核 1=审核通过 2=已到账 3=已拒绝 4=已取消',
    `reviewed_by`  BIGINT       NULL     COMMENT '审核人ID',
    `reviewed_at`  DATETIME     NULL     COMMENT '审核时间',
    `remark`       VARCHAR(512) NOT NULL DEFAULT '' COMMENT '审核备注',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_order_no` (`order_no`),
    KEY `idx_user_time` (`user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='提现订单(分表)';

-- Create remaining shards for recharge_order and withdraw_order
-- Recharge shards 01-15
CREATE TABLE IF NOT EXISTS `recharge_order_01` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_02` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_03` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_04` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_05` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_06` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_07` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_08` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_09` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_10` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_11` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_12` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_13` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_14` LIKE `recharge_order_00`;
CREATE TABLE IF NOT EXISTS `recharge_order_15` LIKE `recharge_order_00`;

-- Withdraw shards 01-15
CREATE TABLE IF NOT EXISTS `withdraw_order_01` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_02` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_03` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_04` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_05` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_06` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_07` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_08` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_09` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_10` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_11` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_12` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_13` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_14` LIKE `withdraw_order_00`;
CREATE TABLE IF NOT EXISTS `withdraw_order_15` LIKE `withdraw_order_00`;
