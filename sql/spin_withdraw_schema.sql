-- ============================================================================
-- Spin Withdraw Module (转盘满额提现系统)
-- 基于 SpinHandler.cpp 逻辑的 Go 实现
-- ============================================================================

-- 转盘提现订单 (Hash Sharded by user_id)
-- 记录用户每次从转盘活动申请提现的订单
CREATE TABLE IF NOT EXISTS `spin_withdraw_order` (
    `id`              BIGINT        NOT NULL AUTO_INCREMENT,
    `user_id`         BIGINT        NOT NULL COMMENT '用户ID',
    `activity_id`     BIGINT        NOT NULL COMMENT '活动ID(关联activity_define)',
    `spin_id`         VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '转盘配置标识(对应C++的spin_id)',
    `order_no`        VARCHAR(64)   NOT NULL COMMENT '平台订单号(snowflake)',
    `amount`          DECIMAL(18,4) NOT NULL COMMENT '提现金额(USD)',
    `flow_required`   DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '所需流水(USD)',
    `nick_name`       VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '用户昵称(冗余快照)',
    `status`          TINYINT       NOT NULL DEFAULT 0 COMMENT '0=待审核 1=自动审核通过 2=人工审核通过 3=审核拒绝 4=已发放',
    `audit_rule_type` TINYINT       NOT NULL DEFAULT 0 COMMENT '命中审核规则: 0=无 1=邀请人充值 2=流水达标 3=下级未充值 4=可疑标签',
    `audit_uid`       BIGINT        NOT NULL DEFAULT 0 COMMENT '审核人ID(0=系统自动)',
    `audit_name`      VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '审核人名称',
    `audit_reason`    VARCHAR(512)  NOT NULL DEFAULT '' COMMENT '审核备注/原因',
    `audit_detail`    JSON          NULL     COMMENT '审核详情JSON(可疑标签、流水、邀请数等)',
    `round`           INT           NOT NULL DEFAULT 0 COMMENT '第几回合',
    `invite_total`    INT           NOT NULL DEFAULT 0 COMMENT '该回合邀请总人数',
    `created_at`       DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`       DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_order_no` (`order_no`),
    KEY `idx_user_time` (`user_id`, `created_at`),
    KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转盘提现订单(分表)';

-- 转盘订单日志 (Hash Sharded by user_id)
-- 记录每个回合结束时的状态快照，用于统计和回溯
CREATE TABLE IF NOT EXISTS `spin_order_log` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`         BIGINT       NOT NULL COMMENT '用户ID',
    `activity_id`     BIGINT       NOT NULL COMMENT '活动ID',
    `spin_id`         VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '转盘标识',
    `round`           INT          NOT NULL DEFAULT 0 COMMENT '回合数',
    `total_amount`    DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '回合累计金额',
    `total_invite`    INT          NOT NULL DEFAULT 0 COMMENT '回合邀请总人数',
    `vip_level`       INT          NOT NULL DEFAULT 0 COMMENT '用户VIP等级',
    `log_type`        TINYINT      NOT NULL DEFAULT 0 COMMENT '0=回合结束快照 1=提现申请',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_activity` (`user_id`, `activity_id`),
    KEY `idx_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转盘订单日志(分表)';

-- 转盘邀请配置 (不分表，由 admin CRUD 管理)
-- 替代 C++ 中的 spin_invite.json 配置，改为数据库配置
CREATE TABLE IF NOT EXISTS `spin_invite_config` (
    `id`              BIGINT       NOT NULL AUTO_INCREMENT,
    `group_id`        INT          NOT NULL COMMENT '邀请组ID(关联activity_define.config_json)',
    `vip_level`       INT          NOT NULL DEFAULT 0 COMMENT 'VIP等级',
    `new_count`       INT          NOT NULL DEFAULT 0 COMMENT '新人保护次数(前N次使用new_ratio)',
    `new_ratio`       INT          NOT NULL DEFAULT 0 COMMENT '新人保护期命中概率(万分比)',
    `default_ratio`   INT          NOT NULL DEFAULT 0 COMMENT '默认命中概率(万分比)',
    `reduce_ratio`    INT          NOT NULL DEFAULT 0 COMMENT '每多邀请一人概率递减(万分比)',
    `base_ratio`      INT          NOT NULL DEFAULT 0 COMMENT '概率下限(万分比)',
    `max_count`       INT          NOT NULL DEFAULT 0 COMMENT '每回合最大邀请次数(达到后100%命中)',
    `max_amount`      DECIMAL(18,4) NOT NULL DEFAULT 0.0000 COMMENT '每回合最大邀请发放金额',
    `status`          TINYINT      NOT NULL DEFAULT 1 COMMENT '1=启用 0=禁用',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_group_vip` (`group_id`, `vip_level`),
    KEY `idx_group` (`group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转盘邀请概率配置';

-- 最近提现成功展示 (不分表)
-- 用于前端展示"最近提现成功"滚动列表
-- 数据来源：从 spin_withdraw_order 中 status=4 的订单聚合
-- 此表作为物化视图缓存，由后台定时任务或审核通过时更新
CREATE TABLE IF NOT EXISTS `spin_top_withdraw` (
    `id`          BIGINT       NOT NULL AUTO_INCREMENT,
    `user_id`     BIGINT       NOT NULL COMMENT '用户ID',
    `nick_name`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '用户昵称',
    `avatar`      VARCHAR(512) NOT NULL DEFAULT '' COMMENT '用户头像',
    `amount`      DECIMAL(18,4) NOT NULL COMMENT '提现金额',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转盘最近提现成功展示';