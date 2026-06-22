-- Role-menu permission mapping table
-- Controls which menu items each role can access.
-- If a role has no rows in this table, it falls back to the minRole level-based system.

CREATE TABLE IF NOT EXISTS `admin_role_menu` (
    `id`        BIGINT      NOT NULL AUTO_INCREMENT,
    `role_id`   BIGINT      NOT NULL COMMENT 'admin_role.id',
    `menu_key`  VARCHAR(64) NOT NULL COMMENT 'Menu key from frontend menu config (e.g. /dashboard, /games/list)',
    `created_at` DATETIME   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_role_menu` (`role_id`, `menu_key`),
    KEY `idx_role_id` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Role menu permission mapping';

-- Seed default menu permissions for built-in roles.
-- super_admin: all menus
-- admin: all menus
-- operator: dashboard, games, activities, orders(recharge only read), ops, system(vip), reports
-- finance: dashboard, orders, reports
-- support: dashboard, users, system(admins read), audit-log
-- viewer: dashboard only

-- super_admin gets all menus
INSERT INTO `admin_role_menu` (`role_id`, `menu_key`)
SELECT r.id, m.menu_key FROM `admin_role` r
CROSS JOIN (
    SELECT '/dashboard' AS menu_key
    UNION SELECT '/users'
    UNION SELECT '/games'
    UNION SELECT '/games/list'
    UNION SELECT '/games/vendors'
    UNION SELECT '/games/categories'
    UNION SELECT '/activities'
    UNION SELECT '/orders'
    UNION SELECT '/orders/recharge'
    UNION SELECT '/orders/withdraw'
    UNION SELECT '/ops'
    UNION SELECT '/ops/banners'
    UNION SELECT '/ops/mail'
    UNION SELECT '/ops/tasks'
    UNION SELECT '/system'
    UNION SELECT '/system/payment'
    UNION SELECT '/system/vip'
    UNION SELECT '/system/admins'
    UNION SELECT '/system/roles'
    UNION SELECT '/audit-log'
    UNION SELECT '/reports'
) m
WHERE r.code = 'super_admin';

-- admin: all menus (same as super_admin)
INSERT INTO `admin_role_menu` (`role_id`, `menu_key`)
SELECT r.id, m.menu_key FROM `admin_role` r
CROSS JOIN (
    SELECT '/dashboard' AS menu_key
    UNION SELECT '/users'
    UNION SELECT '/games'
    UNION SELECT '/games/list'
    UNION SELECT '/games/vendors'
    UNION SELECT '/games/categories'
    UNION SELECT '/activities'
    UNION SELECT '/orders'
    UNION SELECT '/orders/recharge'
    UNION SELECT '/orders/withdraw'
    UNION SELECT '/ops'
    UNION SELECT '/ops/banners'
    UNION SELECT '/ops/mail'
    UNION SELECT '/ops/tasks'
    UNION SELECT '/system'
    UNION SELECT '/system/payment'
    UNION SELECT '/system/vip'
    UNION SELECT '/system/admins'
    UNION SELECT '/system/roles'
    UNION SELECT '/audit-log'
    UNION SELECT '/reports'
) m
WHERE r.code = 'admin';

-- operator
INSERT INTO `admin_role_menu` (`role_id`, `menu_key`)
SELECT r.id, m.menu_key FROM `admin_role` r
CROSS JOIN (
    SELECT '/dashboard' AS menu_key
    UNION SELECT '/games'
    UNION SELECT '/games/list'
    UNION SELECT '/games/vendors'
    UNION SELECT '/games/categories'
    UNION SELECT '/activities'
    UNION SELECT '/orders'
    UNION SELECT '/orders/recharge'
    UNION SELECT '/orders/withdraw'
    UNION SELECT '/ops'
    UNION SELECT '/ops/banners'
    UNION SELECT '/ops/mail'
    UNION SELECT '/ops/tasks'
    UNION SELECT '/system'
    UNION SELECT '/system/vip'
    UNION SELECT '/audit-log'
    UNION SELECT '/reports'
) m
WHERE r.code = 'operator';

-- finance
INSERT INTO `admin_role_menu` (`role_id`, `menu_key`)
SELECT r.id, m.menu_key FROM `admin_role` r
CROSS JOIN (
    SELECT '/dashboard' AS menu_key
    UNION SELECT '/orders'
    UNION SELECT '/orders/recharge'
    UNION SELECT '/orders/withdraw'
    UNION SELECT '/reports'
) m
WHERE r.code = 'finance';

-- support
INSERT INTO `admin_role_menu` (`role_id`, `menu_key`)
SELECT r.id, m.menu_key FROM `admin_role` r
CROSS JOIN (
    SELECT '/dashboard' AS menu_key
    UNION SELECT '/users'
    UNION SELECT '/system'
    UNION SELECT '/system/vip'
    UNION SELECT '/audit-log'
) m
WHERE r.code = 'support';

-- viewer: only dashboard
INSERT INTO `admin_role_menu` (`role_id`, `menu_key`)
SELECT r.id, '/dashboard' AS menu_key FROM `admin_role` r WHERE r.code = 'viewer';