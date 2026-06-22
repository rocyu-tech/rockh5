-- ============================================================================
-- RockGame Database Schema
-- Generation: auto-generated from V5 tech spec
-- Database: MySQL 8.0+
-- Charset: utf8mb4
-- ============================================================================
CREATE DATABASE IF NOT EXISTS `rockgame` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE `rockgame`;

-- ============================================================================
-- 1. Account Module (账户系统)
-- ============================================================================

-- 1.1 用户主表 (不分表)
CREATE TABLE `users` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT COMMENT 'Snowflake UID',
    `email`         VARCHAR(128) NOT NULL DEFAULT '' COMMENT '邮箱',
    `phone`         VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '手机号',
    `phone_code`    VARCHAR(8)   NOT NULL DEFAULT '' COMMENT '国际区号 e.g. +1',
    `password_hash` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'bcrypt hash',
    `nickname`      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '昵称',
    `avatar`        VARCHAR(512) NOT NULL DEFAULT '' COMMENT '头像 CDN URL',
    `status`        TINYINT      NOT NULL DEFAULT 1 COMMENT '0=禁用 1=正常',
    `kyc_status`    TINYINT      NOT NULL DEFAULT 0 COMMENT '0=未提交 1=待审核 2=已通过 3=已拒绝',
    `kyc_level`     TINYINT      NOT NULL DEFAULT 0 COMMENT 'KYC等级 0=无 1=基础 2=高级',
    `language`      VARCHAR(10)  NOT NULL DEFAULT 'en' COMMENT '偏好语言',
    `timezone`      VARCHAR(32)  NOT NULL DEFAULT 'UTC' COMMENT '时区',
    `last_login_at` DATETIME     NULL     COMMENT '最后登录时间',
    `last_login_ip` VARCHAR(45)  NOT NULL DEFAULT '' COMMENT '最后登录IP',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_email` (`email`),
    KEY `idx_phone` (`phone`, `phone_code`),
    KEY `idx_status_created` (`status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户主表';

-- 1.2 KYC (login_log moved to rockgame_log database)

-- 1.3 KYC实名认证
CREATE TABLE `user_kyc` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT       NOT NULL COMMENT '用户ID',
    `id_type`      TINYINT      NOT NULL DEFAULT 0 COMMENT '证件类型 1=身份证 2=护照 3=驾照',
    `id_number`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '证件号(加密存储)',
    `real_name`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '真实姓名(加密存储)',
    `id_images`    JSON         NULL     COMMENT '证件照片URL列表',
    `verify_status` TINYINT      NOT NULL DEFAULT 0 COMMENT '0=待提交 1=审核中 2=通过 3=拒绝',
    `reviewed_by`  BIGINT       NULL     COMMENT '审核人ID',
    `reviewed_at`  DATETIME     NULL     COMMENT '审核时间',
    `remark`       VARCHAR(512) NOT NULL DEFAULT '' COMMENT '审核备注',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='KYC实名认证';

-- 1.4 邀请码
CREATE TABLE `invite_code` (
    `id`          BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`     BIGINT       NOT NULL COMMENT '所属用户ID',
    `code`        VARCHAR(16)  NOT NULL COMMENT '邀请码(8位字母数字)',
    `usage_count` INT          NOT NULL DEFAULT 0 COMMENT '已使用次数',
    `max_usage`   INT          NOT NULL DEFAULT 0 COMMENT '最大使用次数 0=不限',
    `status`      TINYINT      NOT NULL DEFAULT 1 COMMENT '1=有效 0=失效',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_code` (`code`),
    KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='邀请码';

-- 1.5 账号注销申请
CREATE TABLE `account_delete_request` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT       NOT NULL COMMENT '用户ID',
    `reason`       VARCHAR(512) NOT NULL DEFAULT '' COMMENT '注销原因',
    `status`       TINYINT      NOT NULL DEFAULT 0 COMMENT '0=冷静期 1=已撤销 2=已注销',
    `cooling_end`  DATETIME     NULL     COMMENT '冷静期结束时间',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at`   DATETIME     NULL     COMMENT '实际注销时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='账号注销申请';

-- 1.6 用户余额/资产主表 (不分表, 高频读写)
CREATE TABLE `user_wallet` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`         BIGINT       NOT NULL COMMENT '用户ID',
    `cash_balance`    DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '现金余额(USD)',
    `bonus_balance`   DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '赠送余额(USD)',
    `frozen_balance`  DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '冻结余额(USD)',
    `total_recharge`  DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '累计充值(USD)',
    `total_withdraw`  DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '累计提现(USD)',
    `total_win`       DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '累计赢取(USD)',
    `total_bet`       DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '累计下注(USD)',
    `version`        INT          NOT NULL DEFAULT 0 COMMENT '乐观锁版本号',
    `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户钱包';

-- ============================================================================
-- 2. User Referral - Hash Sharded by user_id (16 shards)
-- ============================================================================

CREATE TABLE `user_referral` (
    `id`               BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`          BIGINT       NOT NULL COMMENT '被邀请人ID',
    `inviter_id`       BIGINT       NOT NULL COMMENT '邀请人ID',
    `invite_code`      VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '使用的邀请码',
    `source_channel`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '来源渠道',
    `attribution_data` JSON         NULL     COMMENT '归因数据JSON',
    `campaign`         VARCHAR(128) NOT NULL DEFAULT '' COMMENT '广告系列',
    `ad_id`            VARCHAR(128) NOT NULL DEFAULT '' COMMENT '广告创意ID',
    `status`           TINYINT      NOT NULL DEFAULT 1 COMMENT '1=有效',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user` (`user_id`),
    KEY `idx_inviter` (`inviter_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户归因/邀请关系(分表)';

-- ============================================================================
-- 3. Lobby / CMS Module (大厅CMS)
-- ============================================================================

CREATE TABLE `lobby_config` (
    `id`         BIGINT       NOT NULL AUTO_INCREMENT,
    `lobby_name` VARCHAR(64)  NOT NULL COMMENT '大厅名称',
    `parent_id`  BIGINT       NOT NULL DEFAULT 0 COMMENT '父级ID 0=一级大厅',
    `sort_order` INT          NOT NULL DEFAULT 0 COMMENT '排序权重',
    `status`     TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用 0=禁用',
    `language`   VARCHAR(10)  NOT NULL DEFAULT 'en' COMMENT '语言',
    `icon`       VARCHAR(512) NOT NULL DEFAULT '' COMMENT '大厅图标',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_parent` (`parent_id`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='大厅配置';

CREATE TABLE `banner` (
    `id`          BIGINT       NOT NULL AUTO_INCREMENT,
    `lobby_id`    BIGINT       NOT NULL DEFAULT 0 COMMENT '所属大厅ID 0=全局',
    `image_url`   VARCHAR(512) NOT NULL DEFAULT '' COMMENT '图片CDN URL',
    `link_url`    VARCHAR(512) NOT NULL DEFAULT '' COMMENT '跳转链接',
    `weight`      INT          NOT NULL DEFAULT 0 COMMENT '排序权重',
    `start_time`  DATETIME     NULL     COMMENT '生效开始时间',
    `end_time`    DATETIME     NULL     COMMENT '生效结束时间',
    `target_lang` VARCHAR(10)  NOT NULL DEFAULT '' COMMENT '定向语言 空=全部',
    `status`      TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_lobby_weight` (`lobby_id`, `weight`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Banner轮播图';

CREATE TABLE `splash_popup` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `title`         VARCHAR(128) NOT NULL DEFAULT '' COMMENT '弹窗标题',
    `content`       TEXT         NULL     COMMENT '弹窗内容',
    `image_url`     VARCHAR(512) NOT NULL DEFAULT '' COMMENT '图片URL',
    `link_url`      VARCHAR(512) NOT NULL DEFAULT '' COMMENT '跳转链接',
    `trigger_rules` JSON         NULL     COMMENT '触发规则JSON',
    `daily_limit`   INT          NOT NULL DEFAULT 1 COMMENT '每日展示上限',
    `priority`      INT          NOT NULL DEFAULT 0 COMMENT '优先级',
    `status`        TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_priority` (`priority`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='拍脸弹窗';

CREATE TABLE `game_category` (
    `id`         BIGINT       NOT NULL AUTO_INCREMENT,
    `lobby_id`   BIGINT       NOT NULL DEFAULT 0 COMMENT '所属大厅ID',
    `parent_id`  BIGINT       NOT NULL DEFAULT 0 COMMENT '父分类ID(0=顶级)',
    `name`       VARCHAR(64)  NOT NULL COMMENT '分类名称',
    `icon`       VARCHAR(512) NOT NULL DEFAULT '' COMMENT '分类图标',
    `sort_order` INT          NOT NULL DEFAULT 0 COMMENT '排序',
    `status`     TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_lobby_sort` (`lobby_id`, `sort_order`),
    KEY `idx_parent` (`parent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='游戏分类';

-- ============================================================================
-- 4. Item Module (道具系统)
-- ============================================================================

CREATE TABLE `item_define` (
    `id`          BIGINT       NOT NULL AUTO_INCREMENT,
    `name`        VARCHAR(128) NOT NULL COMMENT '道具名称',
    `icon`        VARCHAR(512) NOT NULL DEFAULT '' COMMENT '道具图标',
    `type`        TINYINT      NOT NULL DEFAULT 0 COMMENT '1=消耗品 2=时效性 3=永久',
    `duration`    INT          NOT NULL DEFAULT 0 COMMENT '有效时长(秒) 0=永久',
    `stackable`   TINYINT      NOT NULL DEFAULT 1 COMMENT '1=可堆叠 0=不可',
    `description` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '描述',
    `i18n_key`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '多语言key',
    `status`      TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_type` (`type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='道具定义';

-- 4.1 user_inventory - Hash Sharded
CREATE TABLE `user_inventory` (
    `id`        BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`   BIGINT       NOT NULL COMMENT '用户ID',
    `item_id`   BIGINT       NOT NULL COMMENT '道具ID',
    `quantity`  INT          NOT NULL DEFAULT 0 COMMENT '数量',
    `source`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '来源 activity/task/shop/mail',
    `expire_at` DATETIME     NULL     COMMENT '过期时间 NULL=永久',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_item` (`user_id`, `item_id`),
    KEY `idx_expire` (`expire_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户道具背包(分表)';

-- 4.2 item_log - Hash Sharded
CREATE TABLE `item_log` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT       NOT NULL COMMENT '用户ID',
    `item_id`      BIGINT       NOT NULL COMMENT '道具ID',
    `change_type`  TINYINT      NOT NULL COMMENT '1=获得 2=消耗 3=过期',
    `change_qty`   INT          NOT NULL COMMENT '变更数量',
    `source`       VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '来源',
    `reference_id` BIGINT       NOT NULL DEFAULT 0 COMMENT '关联ID(订单/活动/任务)',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_time` (`user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='道具变更日志(分表)';

-- ============================================================================
-- 5. Shop Module (商城系统)
-- ============================================================================

CREATE TABLE `payment_channel` (
    `id`                BIGINT       NOT NULL AUTO_INCREMENT,
    `name`              VARCHAR(64)  NOT NULL COMMENT '通道名称',
    `type`              VARCHAR(32)  NOT NULL COMMENT 'stripe/paypal/usdt',
    `config_json`       JSON         NOT NULL COMMENT '通道配置(API key等)',
    `status`            TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `supported_regions` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '支持地区(逗号分隔)',
    `min_amount`        DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '最小金额',
    `max_amount`        DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '最大金额',
    `sort_order`        INT          NOT NULL DEFAULT 0 COMMENT '排序',
    `created_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_type` (`type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='支付通道';

CREATE TABLE `currency_rate` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `from_currency` VARCHAR(8)  NOT NULL COMMENT '源币种',
    `to_currency`  VARCHAR(8)  NOT NULL COMMENT '目标币种',
    `rate`         DECIMAL(18,8) NOT NULL COMMENT '汇率',
    `source`       VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '汇率来源',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_pair` (`from_currency`, `to_currency`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='汇率表';

CREATE TABLE `recharge_bonus_rule` (
    `id`             BIGINT       NOT NULL AUTO_INCREMENT,
    `name`           VARCHAR(128) NOT NULL COMMENT '规则名称',
    `type`           VARCHAR(16)  NOT NULL COMMENT 'first=首充 step=阶梯 limited=限时',
    `conditions_json` JSON         NULL     COMMENT '条件JSON',
    `bonus_rate`     DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT '赠送比例(%)',
    `max_bonus`      DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '最大赠送金额',
    `status`         TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `start_time`     DATETIME     NULL     COMMENT '生效开始',
    `end_time`       DATETIME     NULL     COMMENT '生效结束',
    `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_type_status` (`type`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='充值优惠规则';

-- 5.1 recharge_order - Hash Sharded
CREATE TABLE `recharge_order` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT       NOT NULL COMMENT '用户ID',
    `order_no`      VARCHAR(64)  NOT NULL COMMENT '平台订单号',
    `amount`        DECIMAL(18,4) NOT NULL COMMENT '充值金额(原始币种)',
    `amount_usd`    DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '换算后USD金额',
    `currency`      VARCHAR(8)  NOT NULL DEFAULT 'USD' COMMENT '原始币种',
    `channel_id`    BIGINT       NOT NULL COMMENT '支付通道ID',
    `third_order_no` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '三方订单号',
    `status`        TINYINT      NOT NULL DEFAULT 0 COMMENT '0=待支付 1=已支付 2=失败 3=已退款',
    `paid_at`       DATETIME     NULL     COMMENT '支付完成时间',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_order_no` (`order_no`),
    KEY `idx_user_time` (`user_id`, `created_at`),
    KEY `idx_third_order` (`third_order_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='充值订单(分表)';

-- 5.2 withdraw_order - Hash Sharded
CREATE TABLE `withdraw_order` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT       NOT NULL COMMENT '用户ID',
    `order_no`     VARCHAR(64)  NOT NULL COMMENT '平台订单号',
    `amount`       DECIMAL(18,4) NOT NULL COMMENT '提现金额(USD)',
    `currency`     VARCHAR(8)  NOT NULL DEFAULT 'USD' COMMENT '提现币种',
    `bank_info`    JSON         NOT NULL COMMENT '收款信息(加密)',
    `status`       TINYINT      NOT NULL DEFAULT 0 COMMENT '0=待审核 1=审核通过 2=已到账 3=已拒绝 4=已取消',
    `reviewed_by`  BIGINT       NULL     COMMENT '审核人ID',
    `reviewed_at`  DATETIME     NULL     COMMENT '审核时间',
    `remark`       VARCHAR(512) NOT NULL DEFAULT '' COMMENT '审核备注',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_order_no` (`order_no`),
    KEY `idx_user_time` (`user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='提现订单(分表)';

-- 5.3 user_recharge_bonus - Hash Sharded
CREATE TABLE `user_recharge_bonus` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT       NOT NULL COMMENT '用户ID',
    `order_id`     BIGINT       NOT NULL COMMENT '关联充值订单ID',
    `rule_id`      BIGINT       NOT NULL COMMENT '优惠规则ID',
    `bonus_amount` DECIMAL(18,4) NOT NULL COMMENT '赠送金额(USD)',
    `claimed_at`   DATETIME     NULL     COMMENT '领取时间',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_order` (`user_id`, `order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户充值优惠记录(分表)';

-- ============================================================================
-- 6. Ledger / Accounting (复式记账)
-- ============================================================================

CREATE TABLE `ledger` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT       NOT NULL COMMENT '用户ID',
    `biz_type`      VARCHAR(32)  NOT NULL COMMENT '业务类型 recharge/withdraw/bet/settle/bonus',
    `biz_id`        BIGINT       NOT NULL COMMENT '业务ID(订单号/交易号)',
    `debit_account` VARCHAR(16)  NOT NULL COMMENT '借方科目 cash/bonus/frozen',
    `credit_account` VARCHAR(16) NOT NULL COMMENT '贷方科目 cash/bonus/frozen',
    `amount`        DECIMAL(18,4) NOT NULL COMMENT '金额',
    `balance_after` DECIMAL(18,4) NOT NULL COMMENT '操作后余额',
    `remark`        VARCHAR(256) NOT NULL DEFAULT '' COMMENT '备注',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_biz` (`user_id`, `biz_type`, `biz_id`),
    KEY `idx_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='复式记账流水';

-- ============================================================================
-- 7. VIP Module (VIP系统)
-- ============================================================================

CREATE TABLE `vip_level_config` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `level`           INT          NOT NULL COMMENT 'VIP等级 1-10',
    `name`            VARCHAR(64)  NOT NULL COMMENT '等级名称',
    `growth_required` BIGINT       NOT NULL COMMENT '升级所需成长值',
    `icon`            VARCHAR(512) NOT NULL DEFAULT '' COMMENT '等级图标',
    `withdraw_fee_rate` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT '提现手续费率(%)',
    `daily_signin_bonus` DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '每日签到奖励倍数',
    `benefits_json`   JSON         NULL     COMMENT '权益配置JSON',
    `status`          TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_level` (`level`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='VIP等级配置';

-- 7.1 user_vip - Hash Sharded
CREATE TABLE `user_vip` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT       NOT NULL COMMENT '用户ID',
    `level`         INT          NOT NULL DEFAULT 0 COMMENT '当前VIP等级 0=非VIP',
    `growth`        BIGINT       NOT NULL DEFAULT 0 COMMENT '当前成长值',
    `upgrade_at`    DATETIME     NULL     COMMENT '上次升级时间',
    `last_refresh`  DATETIME     NULL     COMMENT '上次刷新时间',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户VIP(分表)';

-- 7.2 vip_benefit_log - Hash Sharded
CREATE TABLE `vip_benefit_log` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT       NOT NULL COMMENT '用户ID',
    `benefit_type`  VARCHAR(32)  NOT NULL COMMENT '权益类型',
    `benefit_value` DECIMAL(18,4) NOT NULL COMMENT '权益值',
    `source`        VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '来源',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_time` (`user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='VIP权益领取记录(分表)';

-- ============================================================================
-- 8. Task Module (任务系统)
-- ============================================================================

CREATE TABLE `task_define` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `name`         VARCHAR(128) NOT NULL COMMENT '任务名称',
    `type`         VARCHAR(16)  NOT NULL COMMENT 'daily/weekly/growth/limited',
    `cycle`        INT          NOT NULL DEFAULT 1 COMMENT '周期(天)',
    `target_key`   VARCHAR(64)  NOT NULL COMMENT '目标条件key',
    `target_value` INT          NOT NULL COMMENT '目标条件值',
    `reward_type`  VARCHAR(16)  NOT NULL DEFAULT 'coin' COMMENT '奖励类型 coin/item/exp',
    `reward_value` VARCHAR(256) NOT NULL DEFAULT '' COMMENT '奖励值(JSON)',
    `sort_order`   INT          NOT NULL DEFAULT 0 COMMENT '排序',
    `status`       TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_type_status` (`type`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='任务定义';

-- 8.1 user_task - Hash Sharded
CREATE TABLE `user_task` (
    `id`             BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`        BIGINT       NOT NULL COMMENT '用户ID',
    `task_id`        BIGINT       NOT NULL COMMENT '任务定义ID',
    `progress`       INT          NOT NULL DEFAULT 0 COMMENT '当前进度',
    `status`         TINYINT      NOT NULL DEFAULT 0 COMMENT '0=进行中 1=已完成 2=已领取',
    `completed_at`   DATETIME     NULL     COMMENT '完成时间',
    `reward_claimed` TINYINT      NOT NULL DEFAULT 0 COMMENT '0=未领取 1=已领取',
    `cycle_key`      VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '周期标识(20260603)',
    `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_task_cycle` (`user_id`, `task_id`, `cycle_key`),
    KEY `idx_user_status` (`user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户任务进度(分表)';

-- ============================================================================
-- 9. Activity Module (活动系统)
-- ============================================================================

CREATE TABLE `activity_define` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `name`          VARCHAR(128) NOT NULL COMMENT '活动名称',
    `type`          VARCHAR(16)  NOT NULL COMMENT 'signin/wheel/recharge/gift',
    `handler_name`  VARCHAR(64)  NOT NULL COMMENT '插件Handler名称',
    `config_json`   JSON         NOT NULL COMMENT '活动配置JSON',
    `start_time`    DATETIME     NOT NULL COMMENT '开始时间',
    `end_time`      DATETIME     NOT NULL COMMENT '结束时间',
    `status`        TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `priority`      INT          NOT NULL DEFAULT 0 COMMENT '排序优先级',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_type_status` (`type`, `status`),
    KEY `idx_time` (`start_time`, `end_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='活动定义';

CREATE TABLE `activity_inventory` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `activity_id`  BIGINT       NOT NULL COMMENT '活动ID',
    `reward_id`    BIGINT       NOT NULL COMMENT '奖励ID(关联item_define)',
    `total`        INT          NOT NULL DEFAULT 0 COMMENT '总库存',
    `remaining`    INT          NOT NULL DEFAULT 0 COMMENT '剩余库存',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_activity` (`activity_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='活动库存';

-- 9.1 user_activity - Hash Sharded
CREATE TABLE `user_activity` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT       NOT NULL COMMENT '用户ID',
    `activity_id`  BIGINT       NOT NULL COMMENT '活动ID',
    `progress`     INT          NOT NULL DEFAULT 0 COMMENT '进度',
    `state_data`   JSON         NULL     COMMENT '状态数据JSON(签到天数/转盘次数等)',
    `last_draw_at` DATETIME     NULL     COMMENT '上次抽奖时间',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_activity` (`user_id`, `activity_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户活动参与(分表)';

-- 9.2 activity_records - Hash Sharded
CREATE TABLE `activity_records` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT       NOT NULL COMMENT '用户ID',
    `activity_id`  BIGINT       NOT NULL COMMENT '活动ID',
    `action`       VARCHAR(32)  NOT NULL COMMENT '操作 draw/claim/checkin',
    `reward_json`  JSON         NULL     COMMENT '奖励详情',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_activity` (`user_id`, `activity_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='活动操作记录(分表)';

-- ============================================================================
-- 10. Mail Module (邮件/消息系统)
-- ============================================================================

CREATE TABLE `mail_template` (
    `id`       BIGINT       NOT NULL AUTO_INCREMENT,
    `title`    VARCHAR(256) NOT NULL COMMENT '邮件标题',
    `content`  TEXT         NULL     COMMENT '邮件内容(HTML)',
    `i18n_key` VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '多语言key',
    `type`     VARCHAR(16)  NOT NULL DEFAULT 'system' COMMENT 'system/activity/promotion',
    `status`   TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='邮件模板';

-- 10.1 mail - Hash Sharded
CREATE TABLE `mail` (
    `id`             BIGINT       NOT NULL AUTO_INCREMENT,
    `receiver_id`    BIGINT       NOT NULL COMMENT '收件人ID',
    `sender_type`    TINYINT      NOT NULL DEFAULT 0 COMMENT '0=系统 1=管理员 2=代理',
    `sender_id`      BIGINT       NOT NULL DEFAULT 0 COMMENT '发送人ID',
    `title`          VARCHAR(256) NOT NULL COMMENT '标题',
    `content`        TEXT         NULL     COMMENT '内容',
    `attachment_json` JSON         NULL     COMMENT '附件JSON(金币/道具)',
    `push_channel`   VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '推送通道 push/email',
    `push_status`    TINYINT      NOT NULL DEFAULT 0 COMMENT '推送状态 0=未推送 1=成功 2=失败',
    `email_status`    TINYINT      NOT NULL DEFAULT 0 COMMENT '邮件状态',
    `status`         TINYINT      NOT NULL DEFAULT 0 COMMENT '0=未读 1=已读 2=已删除',
    `expire_at`       DATETIME     NULL     COMMENT '过期时间',
    `read_at`         DATETIME     NULL     COMMENT '阅读时间',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_receiver_status` (`receiver_id`, `status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='站内信(分表)';

CREATE TABLE `push_token` (
    `id`         BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`    BIGINT       NOT NULL COMMENT '用户ID',
    `platform`   VARCHAR(16)  NOT NULL COMMENT 'ios/android',
    `token`      VARCHAR(512) NOT NULL COMMENT 'FCM/APNs Token',
    `device_id`  VARCHAR(128) NOT NULL DEFAULT '' COMMENT '设备ID',
    `status`     TINYINT      NOT NULL DEFAULT 1 COMMENT '1=有效',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_device` (`user_id`, `device_id`),
    KEY `idx_token` (`token`(128))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='设备推送Token';

CREATE TABLE `mail_queue` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `mail_id`      BIGINT       NOT NULL COMMENT '邮件ID',
    `channel`      VARCHAR(16)  NOT NULL COMMENT 'in_app/push/email',
    `priority`     INT          NOT NULL DEFAULT 0 COMMENT '优先级',
    `retry_count`  INT          NOT NULL DEFAULT 0 COMMENT '重试次数',
    `status`       TINYINT      NOT NULL DEFAULT 0 COMMENT '0=待发送 1=已发送 2=失败',
    `next_retry_at` DATETIME     NULL     COMMENT '下次重试时间',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_status_retry` (`status`, `next_retry_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='邮件发送队列';

-- ============================================================================
-- 11. Rank Module (排行榜系统)
-- ============================================================================

CREATE TABLE `rank_define` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `name`          VARCHAR(128) NOT NULL COMMENT '排行榜名称',
    `type`          VARCHAR(16)  NOT NULL COMMENT 'recharge/win/agent/activity',
    `period`        VARCHAR(16)  NOT NULL COMMENT 'daily/weekly/monthly/all',
    `reset_rule`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '重置规则cron',
    `reward_config` JSON         NULL     COMMENT '排名奖励配置',
    `status`        TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_type_period` (`type`, `period`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='排行榜定义';

CREATE TABLE `rank_snapshot` (
    `id`             BIGINT       NOT NULL AUTO_INCREMENT,
    `rank_define_id` BIGINT       NOT NULL COMMENT '排行榜定义ID',
    `period_key`     VARCHAR(16)  NOT NULL COMMENT '周期标识 20260603',
    `user_id`        BIGINT       NOT NULL COMMENT '用户ID',
    `score`          DECIMAL(18,4) NOT NULL COMMENT '分数',
    `rank`           INT          NOT NULL COMMENT '排名',
    `snapshot_time`  DATETIME     NOT NULL COMMENT '快照时间',
    PRIMARY KEY (`id`),
    KEY `idx_define_period` (`rank_define_id`, `period_key`),
    KEY `idx_user_rank` (`user_id`, `rank`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='排行榜历史快照';

-- ============================================================================
-- 12. Agent Module (代理分销系统)
-- ============================================================================

CREATE TABLE `agent` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`         BIGINT       NOT NULL COMMENT '用户ID',
    `parent_id`       BIGINT       NOT NULL DEFAULT 0 COMMENT '上级代理ID 0=顶级',
    `level`           INT          NOT NULL DEFAULT 1 COMMENT '代理等级 1/2/3',
    `status`          TINYINT      NOT NULL DEFAULT 1 COMMENT '0=禁用 1=正常 2=待审核',
    `commission_rate` DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT '佣金比例(%)',
    `promo_code`      VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '推广码',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user` (`user_id`),
    UNIQUE KEY `uk_promo` (`promo_code`),
    KEY `idx_parent` (`parent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理信息';

CREATE TABLE `agent_stats` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `agent_id`        BIGINT       NOT NULL COMMENT '代理ID',
    `period`          VARCHAR(16)  NOT NULL COMMENT '周期 daily/weekly/monthly',
    `period_key`      VARCHAR(16)  NOT NULL COMMENT '周期标识 20260603',
    `promote_count`   INT          NOT NULL DEFAULT 0 COMMENT '推广人数',
    `active_count`    INT          NOT NULL DEFAULT 0 COMMENT '活跃人数',
    `recharge_total`  DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '下级充值总额',
    `commission_total` DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '佣金总额',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_agent_period` (`agent_id`, `period`, `period_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理统计';

CREATE TABLE `promo_link` (
    `id`                 BIGINT       NOT NULL AUTO_INCREMENT,
    `agent_id`           BIGINT       NOT NULL COMMENT '代理ID',
    `short_code`         VARCHAR(16)  NOT NULL COMMENT '短链码',
    `target_url`         VARCHAR(512) NOT NULL COMMENT '目标URL',
    `utm_params`         JSON         NULL     COMMENT 'UTM参数',
    `landing_template`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '落地页模板',
    `click_count`        BIGINT       NOT NULL DEFAULT 0 COMMENT '点击次数',
    `created_at`          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_short_code` (`short_code`),
    KEY `idx_agent` (`agent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='推广链接';

-- 12.1 agent_commission - Hash Sharded
CREATE TABLE `agent_commission` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `agent_id`     BIGINT       NOT NULL COMMENT '代理ID',
    `from_user_id` BIGINT       NOT NULL COMMENT '来源用户ID',
    `amount`       DECIMAL(18,4) NOT NULL COMMENT '佣金金额',
    `rate`         DECIMAL(5,2) NOT NULL COMMENT '佣金比例(%)',
    `status`       TINYINT      NOT NULL DEFAULT 0 COMMENT '0=待结算 1=已结算 2=已提现',
    `settled_at`   DATETIME     NULL     COMMENT '结算时间',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_agent_status` (`agent_id`, `status`),
    KEY `idx_agent_time` (`agent_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='代理佣金记录(分表)';

-- ============================================================================
-- 13. Game Module (三方游戏接入系统)
-- ============================================================================

CREATE TABLE `game_vendor` (
    `id`             BIGINT       NOT NULL AUTO_INCREMENT,
    `name`           VARCHAR(64)  NOT NULL COMMENT '厂商名称 e.g. PG/PP/EVO',
    `logo`           VARCHAR(512) NOT NULL DEFAULT '' COMMENT '厂商Logo',
    `api_key`        VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'API Key(加密)',
    `api_secret`     VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'API Secret(加密)',
    `callback_url`   VARCHAR(512) NOT NULL DEFAULT '' COMMENT '回调地址',
    `status`         TINYINT      NOT NULL DEFAULT 1 COMMENT '1=正常 0=维护中',
    `timeout_config` JSON         NULL     COMMENT '超时配置JSON',
    `retry_policy`   JSON         NULL     COMMENT '重试策略JSON',
    `error_threshold` INT         NOT NULL DEFAULT 30 COMMENT '错误率阈值(%)',
    `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='游戏厂商';

CREATE TABLE `game_info` (
    `id`          BIGINT       NOT NULL AUTO_INCREMENT,
    `vendor_id`   BIGINT       NOT NULL COMMENT '厂商ID',
    `game_id`     VARCHAR(64)  NOT NULL COMMENT '厂商游戏ID',
    `name`        VARCHAR(128) NOT NULL COMMENT '游戏名称',
    `icon`        VARCHAR(512) NOT NULL DEFAULT '' COMMENT '游戏图标',
    `url`         VARCHAR(512) NOT NULL DEFAULT '' COMMENT '游戏URL',
    `category_id` BIGINT       NOT NULL DEFAULT 0 COMMENT '分类ID',
    `rtp`         DECIMAL(5,2) NOT NULL DEFAULT 0 COMMENT 'RTP(%)',
    `bet_min`     DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '最小下注',
    `bet_max`     DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '最大下注',
    `status`      TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用 0=下架',
    `sort_order`  INT          NOT NULL DEFAULT 0 COMMENT '排序权重',
    `hot`         TINYINT      NOT NULL DEFAULT 0 COMMENT '1=热门',
    `new`         TINYINT      NOT NULL DEFAULT 0 COMMENT '1=新游戏',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_vendor_game` (`vendor_id`, `game_id`),
    KEY `idx_category` (`category_id`),
    KEY `idx_status_sort` (`status`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='游戏列表';

CREATE TABLE `game_session` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`         BIGINT       NOT NULL COMMENT '用户ID',
    `game_info_id`    BIGINT       NOT NULL COMMENT '游戏ID',
    `session_token`   VARCHAR(256) NOT NULL COMMENT '会话JWT Token',
    `balance_snapshot` DECIMAL(18,4) NOT NULL COMMENT '进入时余额快照',
    `status`          TINYINT      NOT NULL DEFAULT 1 COMMENT '1=进行中 0=已结束',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `ended_at`         DATETIME     NULL     COMMENT '结束时间',
    PRIMARY KEY (`id`),
    KEY `idx_user_status` (`user_id`, `status`),
    KEY `idx_token` (`session_token`(128))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='游戏会话';

CREATE TABLE `game_rtp_config` (
    `id`             BIGINT        NOT NULL AUTO_INCREMENT,
    `game_info_id`   BIGINT        NOT NULL COMMENT '游戏ID',
    `min_rtp`        DECIMAL(5,2)  NOT NULL DEFAULT 96.00 COMMENT '最小RTP(%)',
    `max_rtp`        DECIMAL(5,2)  NOT NULL DEFAULT 98.00 COMMENT '最大RTP(%)',
    `min_bet`        DECIMAL(18,4) NOT NULL DEFAULT 0.1000 COMMENT '最小下注额',
    `max_bet`        DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '最大下注额',
    `daily_loss_limit` DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '日累计亏损限额',
    `max_win_limit`  DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '单局最大赢取',
    `status`         TINYINT       NOT NULL DEFAULT 1 COMMENT '1=启用',
    `created_at`      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_game` (`game_info_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='游戏RTP/限红配置';

-- 13.1 game_transaction - Hash Sharded
CREATE TABLE `game_transaction` (
    `id`             BIGINT       NOT NULL AUTO_INCREMENT,
    `session_id`     BIGINT       NOT NULL COMMENT '游戏会话ID',
    `user_id`        BIGINT       NOT NULL COMMENT '用户ID',
    `vendor_id`      BIGINT       NOT NULL COMMENT '厂商ID',
    `game_info_id`   BIGINT       NOT NULL COMMENT '游戏ID',
    `type`           VARCHAR(16)  NOT NULL COMMENT 'bet/settle/cancel',
    `amount`         DECIMAL(18,4) NOT NULL COMMENT '金额',
    `vendor_tx_id`   VARCHAR(128) NOT NULL DEFAULT '' COMMENT '厂商交易号',
    `platform_tx_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '平台交易号',
    `balance_before` DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '交易前余额',
    `balance_after`  DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '交易后余额',
    `status`         TINYINT      NOT NULL DEFAULT 0 COMMENT '0=处理中 1=成功 2=失败',
    `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_platform_tx` (`platform_tx_id`),
    KEY `idx_user_time` (`user_id`, `created_at`),
    KEY `idx_vendor_tx` (`vendor_id`, `vendor_tx_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='游戏交易记录(分表)';

-- ============================================================================
-- 14. Red Dot Module (红点通知系统)
-- ============================================================================

CREATE TABLE `red_dot_config` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `path`         VARCHAR(128) NOT NULL COMMENT '红点路径',
    `parent_path`  VARCHAR(128) NOT NULL DEFAULT '' COMMENT '父节点路径',
    `display_name` VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '显示名称',
    `i18n_key`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '多语言key',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_path` (`path`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='红点配置';

-- 14.1 user_red_dot - Hash Sharded
CREATE TABLE `user_red_dot` (
    `id`         BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`    BIGINT       NOT NULL COMMENT '用户ID',
    `path`       VARCHAR(128) NOT NULL COMMENT '红点路径',
    `has_dot`    TINYINT      NOT NULL DEFAULT 0 COMMENT '0=无红点 1=有红点',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_path` (`user_id`, `path`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户红点状态(分表)';

-- ============================================================================
-- 15. Event Module (事件统计系统)
-- ============================================================================

CREATE TABLE `event_define` (
    `id`          BIGINT       NOT NULL AUTO_INCREMENT,
    `event_name`  VARCHAR(64)  NOT NULL COMMENT '事件名称',
    `category`    VARCHAR(32)  NOT NULL COMMENT '事件分类 user/payment/game/activity',
    `description` VARCHAR(256) NOT NULL DEFAULT '' COMMENT '描述',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_event` (`event_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事件定义';

-- NOTE: event_log, click_log, stat_snapshot, stat_report, admin_audit_log
--       moved to rockgame_log database (sql/schema_log.sql)

-- ============================================================================
-- 16. Tag Module (标签系统)
-- ============================================================================

CREATE TABLE `tag_define` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `name`          VARCHAR(64)  NOT NULL COMMENT '标签名称',
    `type`          VARCHAR(16)  NOT NULL COMMENT 'auto/manual',
    `rule_config`   JSON         NULL     COMMENT '自动标签规则JSON',
    `refresh_cycle`  VARCHAR(16)  NOT NULL DEFAULT 'daily' COMMENT '刷新周期',
    `description`   VARCHAR(256) NOT NULL DEFAULT '' COMMENT '描述',
    `status`        TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用',
    `created_at`     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_type` (`type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='标签定义';

-- 16.1 user_tag - Hash Sharded
CREATE TABLE `user_tag` (
    `id`        BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`   BIGINT       NOT NULL COMMENT '用户ID',
    `tag_id`    BIGINT       NOT NULL COMMENT '标签ID',
    `tagged_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_tag` (`user_id`, `tag_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户标签(分表)';

CREATE TABLE `tag_group` (
    `id`          BIGINT       NOT NULL AUTO_INCREMENT,
    `name`        VARCHAR(128) NOT NULL COMMENT '标签组名称',
    `operator`    VARCHAR(8)   NOT NULL COMMENT 'AND/OR/NOT',
    `tag_ids`     JSON         NOT NULL COMMENT '标签ID列表',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='标签组(圈人)';

-- ============================================================================
-- 17. Admin Module (后台管理)
-- ============================================================================

CREATE TABLE `admin_user` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `username`      VARCHAR(64)  NOT NULL COMMENT '管理员账号',
    `password_hash` VARCHAR(128) NOT NULL COMMENT '密码hash',
    `real_name`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '显示名称',
    `email`         VARCHAR(128) NOT NULL DEFAULT '' COMMENT '邮箱',
    `role`          VARCHAR(32)  NOT NULL DEFAULT 'operator' COMMENT '角色 super/admin/operator/viewer',
    `status`        TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用 0=禁用',
    `last_login_at` DATETIME     NULL     COMMENT '最后登录时间',
    `last_login_ip` VARCHAR(45)  NOT NULL DEFAULT '' COMMENT '最后登录IP',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_username` (`username`),
    UNIQUE KEY `uk_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员账号';

-- admin_audit_log moved to rockgame_log database (sql/schema_log.sql)


