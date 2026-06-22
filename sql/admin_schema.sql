USE `rockgame`;

-- Admin users table
CREATE TABLE IF NOT EXISTS `admin_user` (
    `id`            BIGINT       NOT NULL AUTO_INCREMENT,
    `username`      VARCHAR(64)  NOT NULL COMMENT 'Username',
    `password_hash` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'bcrypt hash',
    `real_name`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT 'Display name',
    `email`         VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Email',
    `role`          VARCHAR(32)  NOT NULL DEFAULT 'operator' COMMENT 'super/admin/operator/viewer',
    `status`        TINYINT      NOT NULL DEFAULT 1 COMMENT '1=active 0=disabled',
    `last_login_at` DATETIME     NULL     COMMENT 'Last login time',
    `last_login_ip` VARCHAR(45)  NOT NULL DEFAULT '' COMMENT 'Last login IP',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_username` (`username`),
    UNIQUE KEY `uk_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Admin users';

-- Admin audit log
CREATE TABLE IF NOT EXISTS `admin_audit_log` (
    `id`           BIGINT       NOT NULL AUTO_INCREMENT,
    `admin_id`     BIGINT       NOT NULL COMMENT 'Admin user ID',
    `admin_name`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT 'Admin username',
    `action`       VARCHAR(64)  NOT NULL COMMENT 'Action: login/user.ban/order.approve etc.',
    `target_type`  VARCHAR(32)  NOT NULL DEFAULT '' COMMENT 'Target type: user/order/activity etc.',
    `target_id`    VARCHAR(64)  NOT NULL DEFAULT '' COMMENT 'Target ID',
    `detail`       TEXT         NULL     COMMENT 'Action detail (JSON or text)',
    `ip`           VARCHAR(45)  NOT NULL DEFAULT '' COMMENT 'Client IP',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_admin_time` (`admin_id`, `created_at`),
    KEY `idx_action` (`action`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Admin operation audit log';
