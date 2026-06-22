-- Admin role management table
-- Roles are referenced by admin_user.role (string code).
-- Built-in roles (is_system=1) cannot be deleted.

CREATE TABLE IF NOT EXISTS `admin_role` (
    `id`          BIGINT       NOT NULL AUTO_INCREMENT,
    `code`        VARCHAR(32)  NOT NULL COMMENT 'Role code used in admin_user.role',
    `name`        VARCHAR(64)  NOT NULL COMMENT 'Display name',
    `level`       INT          NOT NULL DEFAULT 10 COMMENT 'Permission level (higher = more access)',
    `description` VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'Role description',
    `is_system`   TINYINT      NOT NULL DEFAULT 0 COMMENT '1=built-in role, cannot delete',
    `status`      TINYINT      NOT NULL DEFAULT 1 COMMENT '1=active 0=disabled',
    `sort_order`  INT          NOT NULL DEFAULT 0 COMMENT 'Display sort order',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Admin role definitions';

-- Seed built-in roles
INSERT INTO `admin_role` (`code`, `name`, `level`, `description`, `is_system`, `sort_order`) VALUES
('super_admin', 'Super Admin', 100, 'Full system access, can manage all admins and system config', 1, 1),
('admin',       'Admin',       80,  'Can manage most resources except system config', 1, 2),
('operator',    'Operator',    50,  'Can manage games, banners, activities, tasks', 1, 3),
('finance',     'Finance',     40,  'Can manage recharge/withdraw orders and reports', 1, 4),
('support',     'Support',     20,  'Can view users and audit logs', 1, 5),
('viewer',      'Viewer',      10,  'Read-only access to dashboard', 1, 6);