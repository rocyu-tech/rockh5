-- ============================================================================
-- RockGame Log Database Schema
-- Separate database for log/stat/audit tables to isolate high-volume writes
-- from core business tables.
-- Database: MySQL 8.0+
-- Charset: utf8mb4
-- ============================================================================
CREATE DATABASE IF NOT EXISTS `rockgame_log` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE `rockgame_log`;

-- ============================================================================
-- 1. Event Log (按日分表, event_log_YYYYMMDD)
-- 高频事件日志：用户注册、登录、充值、游戏等
-- ============================================================================

CREATE TABLE `event_log` (
    `id`             BIGINT       NOT NULL AUTO_INCREMENT,
    `event_name`     VARCHAR(64)  NOT NULL COMMENT '事件名称',
    `user_id`        BIGINT       NOT NULL COMMENT '用户ID',
    `properties_json` JSON         NULL     COMMENT '事件属性JSON',
    `timestamp`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_event_time` (`event_name`, `timestamp`),
    KEY `idx_user_time` (`user_id`, `timestamp`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事件日志(按日分表, base table for LIKE)';

-- ============================================================================
-- 2. Click Log (按日分表, click_log_YYYYMMDD)
-- 推广链接点击追踪日志
-- ============================================================================

CREATE TABLE `click_log` (
    `id`             BIGINT       NOT NULL AUTO_INCREMENT,
    `promo_link_id`  BIGINT       NOT NULL COMMENT '推广链接ID',
    `ip`             VARCHAR(45)  NOT NULL DEFAULT '' COMMENT '点击IP',
    `user_agent`     VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'UA',
    `country`        VARCHAR(8)   NOT NULL DEFAULT '' COMMENT '国家代码',
    `converted`      TINYINT      NOT NULL DEFAULT 0 COMMENT '是否转化 0=否 1=是',
    `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_promo_created` (`promo_link_id`, `created_at`),
    KEY `idx_country` (`country`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='点击日志(按日分表, base table for LIKE)';

-- ============================================================================
-- 3. Stat Snapshot (统计快照)
-- 实时/日/周/月维度的统计指标快照
-- ============================================================================

CREATE TABLE `stat_snapshot` (
    `id`          BIGINT       NOT NULL AUTO_INCREMENT,
    `stat_key`    VARCHAR(128) NOT NULL COMMENT '指标key e.g. dau, revenue_daily',
    `stat_value`  DECIMAL(18,4) NOT NULL COMMENT '指标值',
    `period_type` VARCHAR(16)  NOT NULL COMMENT 'realtime/daily/weekly/monthly',
    `period_key`  VARCHAR(16)  NOT NULL COMMENT '周期标识 e.g. 20260605',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_key_period` (`stat_key`, `period_type`, `period_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='统计快照';

-- ============================================================================
-- 4. Stat Report (统计报表)
-- 聚合后的统计报表JSON
-- ============================================================================

CREATE TABLE `stat_report` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `report_type`   VARCHAR(32)  NOT NULL COMMENT '报表类型 e.g. revenue, retention',
    `data_json`     JSON         NOT NULL COMMENT '报表数据JSON',
    `generated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_type_time` (`report_type`, `generated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='统计报表';

-- ============================================================================
-- 5. Admin Audit Log (管理员操作日志)
-- 后台管理操作审计追踪
-- ============================================================================

CREATE TABLE `admin_audit_log` (
    `id`         BIGINT       NOT NULL AUTO_INCREMENT,
    `admin_id`   BIGINT       NOT NULL COMMENT '管理员ID',
    `action`     VARCHAR(64)  NOT NULL COMMENT '操作 e.g. user.ban, game.config',
    `target`     VARCHAR(256) NOT NULL DEFAULT '' COMMENT '操作对象',
    `detail`     TEXT         NULL     COMMENT '操作详情',
    `ip`         VARCHAR(45)  NOT NULL DEFAULT '' COMMENT 'IP',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_admin_time` (`admin_id`, `created_at`),
    KEY `idx_action_time` (`action`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员操作日志';

-- ============================================================================
-- 6. User Login Log (用户登录日志)
-- 用户登录/登出记录
-- ============================================================================

CREATE TABLE `user_login_log` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT       NOT NULL COMMENT '用户ID',
    `ip`           VARCHAR(45)  NOT NULL DEFAULT '' COMMENT '登录IP',
    `device_id`    VARCHAR(128) NOT NULL DEFAULT '' COMMENT '设备指纹',
    `user_agent`   VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'UA',
    `login_result` TINYINT      NOT NULL DEFAULT 1 COMMENT '1=成功 0=失败',
    `login_time`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_time` (`user_id`, `login_time`),
    KEY `idx_login_time` (`login_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户登录日志';
