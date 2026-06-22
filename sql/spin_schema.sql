-- Spin Wheel (转盘提现) Tables
-- Ported from C++ SpinHandler.cpp data structures
-- These tables support the spin wheel feature where users accumulate
-- amount through daily free spins and friend invitations, then withdraw.

-- 1. Spin Config: main wheel configuration (one per spin variant)
CREATE TABLE IF NOT EXISTS `spin_config` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `spin_id` VARCHAR(32) NOT NULL COMMENT 'Unique spin identifier (e.g. "1", "vip_special")',
  `full_gold` BIGINT NOT NULL DEFAULT 0 COMMENT 'Target amount to fill the wheel',
  `flow_multi` INT NOT NULL DEFAULT 0 COMMENT 'Flow multiplier for withdrawal (RATIO_BASE=10000)',
  `time_limit_hour` INT NOT NULL DEFAULT 72 COMMENT 'Round duration in hours',
  `audit_usercnt` INT NOT NULL DEFAULT -1 COMMENT 'Audit rule 1: check last N invitees (-1=off)',
  `audit_recharge` BIGINT NOT NULL DEFAULT 0 COMMENT 'Audit rule 1: auto-pass threshold (deprecated)',
  `audit_rule_2_invitetotal_lt` INT NOT NULL DEFAULT 0 COMMENT 'Audit rule 2: invite total < N',
  `audit_rule_2_flowmutil` BIGINT NOT NULL DEFAULT 0 COMMENT 'Audit rule 2: valid flow multiplier',
  `audit_rule_3_invtetotal_ge` INT NOT NULL DEFAULT 0 COMMENT 'Audit rule 3: invite total >= N (auto-reject non-rechargers)',
  `audit_rule_4_users` INT NOT NULL DEFAULT -1 COMMENT 'Audit rule 4: suspect label count threshold (-1=off)',
  `audit_rule_4_labels` VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'Audit rule 4: comma-separated suspect label IDs',
  `start_time` BIGINT NOT NULL DEFAULT 0 COMMENT 'Activity start timestamp (Unix)',
  `end_time` BIGINT NOT NULL DEFAULT 0 COMMENT 'Activity end timestamp (Unix)',
  `user_type` TINYINT NOT NULL DEFAULT 0 COMMENT '0=all(exclude tags), 1=partial(show tags), 2=specific UIDs',
  `tag_list` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'Comma-separated label IDs for targeting',
  `user_list` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'Comma-separated user IDs for targeting',
  `plot_list` VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'Comma-separated plot IDs associated with this spin',
  `invite_group_id` INT NOT NULL DEFAULT 0 COMMENT 'Which invite config group to use',
  `priority` INT NOT NULL DEFAULT 0 COMMENT 'Higher = higher priority for matching',
  `box_gt` INT NOT NULL DEFAULT 0 COMMENT 'Gift box range lower bound (exclusive)',
  `box_le` INT NOT NULL DEFAULT 0 COMMENT 'Gift box range upper bound (inclusive)',
  `items_json` TEXT NOT NULL COMMENT 'JSON array of SpinItem: [{id,prop_id,num_gt,num_le}]',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=active, 0=disabled',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_spin_id` (`spin_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Spin wheel configuration';

-- 2. Spin Plot Config: plot/script configurations that control the amount sequence
CREATE TABLE IF NOT EXISTS `spin_plot_config` (
  `id` INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `step_inc` INT NOT NULL DEFAULT 0 COMMENT 'Default increment after all plot steps exhausted',
  `free_inc` TEXT NOT NULL COMMENT 'JSON array of cumulative amounts: [500,800,1200,...]',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=active, 0=disabled',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Spin plot/script configuration';

-- 3. Spin Invite Config: invite probability settings per VIP level per group
CREATE TABLE IF NOT EXISTS `spin_invite_config` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `group_id` INT NOT NULL COMMENT 'Invite config group ID (links to spin_config.invite_group_id)',
  `vip` INT NOT NULL DEFAULT 0 COMMENT 'VIP level this config applies to',
  `new_count` INT NOT NULL DEFAULT 0 COMMENT 'First N invites use new_ratio',
  `new_ratio` INT NOT NULL DEFAULT 0 COMMENT 'Hit probability for first N invites (RATIO_BASE=10000)',
  `default_ratio` INT NOT NULL DEFAULT 0 COMMENT 'Base hit probability after new_count',
  `reduce_ratio` INT NOT NULL DEFAULT 0 COMMENT 'Probability reduction per invite after new_count',
  `base_ratio` INT NOT NULL DEFAULT 0 COMMENT 'Minimum hit probability floor',
  `max_count` INT NOT NULL DEFAULT 0 COMMENT 'After this many invites, guaranteed hit',
  `max_amount` BIGINT NOT NULL DEFAULT 0 COMMENT 'Reserved field (unused in current logic)',
  PRIMARY KEY (`id`),
  KEY `idx_group_id` (`group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Spin invite probability configuration';

-- 4. Spin Poster Config: poster sharing configurations per language
CREATE TABLE IF NOT EXISTS `spin_poster_config` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `language` VARCHAR(10) NOT NULL DEFAULT 'en' COMMENT 'Language code (en, pt, etc.)',
  `share_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'Share URL template (#code# = invite code)',
  `telegram_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'Telegram share URL template',
  `whatsapp_url` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'WhatsApp share URL template',
  `share_url_prefix` VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'Share text prefix',
  `posters_json` TEXT NOT NULL COMMENT 'JSON array of poster items',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=active, 0=disabled',
  PRIMARY KEY (`id`),
  KEY `idx_language` (`language`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Spin poster sharing configuration';

-- 5. User Spin Data: per-user spin wheel state (one row per user)
CREATE TABLE IF NOT EXISTS `user_spin_data` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT NOT NULL COMMENT 'User ID',
  `spin_id` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'Current spin config ID',
  `cur_round` INT NOT NULL DEFAULT 0 COMMENT 'Current round number',
  `round_start_ts` BIGINT NOT NULL DEFAULT 0 COMMENT 'Round start timestamp (Unix)',
  `cur_amount` BIGINT NOT NULL DEFAULT 0 COMMENT 'Current accumulated amount this round',
  `cur_plot_step` INT NOT NULL DEFAULT 0 COMMENT 'Current position in the plot script',
  `plot_id` INT NOT NULL DEFAULT 0 COMMENT 'Which plot script is active',
  `free_times` INT NOT NULL DEFAULT 1 COMMENT 'Remaining daily free spins',
  `last_free_spin_ts` BIGINT NOT NULL DEFAULT 0 COMMENT 'Last free spin timestamp (Unix)',
  `invite_count` INT NOT NULL DEFAULT 0 COMMENT 'Invites this round',
  `total_invite` INT NOT NULL DEFAULT 0 COMMENT 'Lifetime total invites',
  `level_invite` INT NOT NULL DEFAULT 0 COMMENT 'Invites since last VIP upgrade',
  `total_withdraw` BIGINT NOT NULL DEFAULT 0 COMMENT 'Lifetime total withdrawn amount',
  `round_record` TEXT NOT NULL COMMENT 'JSON array of spin records this round',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Per-user spin wheel state';

-- 6. Spin Withdraw Order: withdrawal requests
CREATE TABLE IF NOT EXISTS `spin_withdraw_order` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_no` VARCHAR(64) NOT NULL COMMENT 'Unique order number',
  `user_id` BIGINT NOT NULL COMMENT 'User ID',
  `nick_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'User nickname at time of request',
  `amount` BIGINT NOT NULL DEFAULT 0 COMMENT 'Withdrawal amount (internal units)',
  `flow` BIGINT NOT NULL DEFAULT 0 COMMENT 'Required flow for this withdrawal',
  `round` INT NOT NULL DEFAULT 0 COMMENT 'Spin round number',
  `spin_id` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'Spin config ID',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '0=pending, 1=approved, 2=delayed, 3=rejected',
  `audit_uid` BIGINT NOT NULL DEFAULT 0 COMMENT 'Admin/user ID who audited',
  `audit_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'Admin name who audited',
  `reason` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'Audit reason (for rejection)',
  `audit_rule_type` INT NOT NULL DEFAULT 0 COMMENT 'Which audit rule decided the outcome',
  `audit_json` TEXT NOT NULL COMMENT 'JSON with full audit data',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Spin withdrawal orders';

-- 7. Spin Order Log: order lifecycle audit trail
CREATE TABLE IF NOT EXISTS `spin_order_log` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `order_id` BIGINT NOT NULL COMMENT 'Associated spin_withdraw_order.id',
  `user_id` BIGINT NOT NULL COMMENT 'User ID',
  `type` TINYINT NOT NULL DEFAULT 1 COMMENT '1=withdraw request, 2=audit action',
  `detail` TEXT NOT NULL COMMENT 'Human-readable log detail',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_order_id` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Spin order audit logs';

-- ── Seed Data: Default spin config for testing ──

-- Default plot script: 16-step progression to ~100000
INSERT INTO `spin_plot_config` (`step_inc`, `free_inc`, `status`) VALUES
(20, '[500, 800, 1200, 1500, 2000, 3000, 5000, 8000, 12000, 18000, 25000, 35000, 50000, 70000, 90000, 98000]', 1);

-- Default spin config
INSERT INTO `spin_config` (
  `spin_id`, `full_gold`, `flow_multi`, `time_limit_hour`,
  `audit_usercnt`, `audit_rule_2_invitetotal_lt`, `audit_rule_2_flowmutil`,
  `audit_rule_3_invtetotal_ge`, `audit_rule_4_users`, `audit_rule_4_labels`,
  `start_time`, `end_time`, `user_type`, `plot_list`, `invite_group_id`,
  `priority`, `box_gt`, `box_le`, `items_json`, `status`
) VALUES (
  '1', 100000, 3000, 72,
  5, 10, 3000,
  5, 2, '101,102,103',
  UNIX_TIMESTAMP(), UNIX_TIMESTAMP() + 365*86400, 0, '1', 1,
  10, 100, 500,
  '[{"id":1,"prop_id":101,"num_gt":0,"num_le":500},{"id":2,"prop_id":102,"num_gt":500,"num_le":2000},{"id":3,"prop_id":103,"num_gt":2000,"num_le":5000},{"id":4,"prop_id":104,"num_gt":5000,"num_le":10000}]',
  1
);

-- Default invite config (group 1, VIP 0)
INSERT INTO `spin_invite_config` (`group_id`, `vip`, `new_count`, `new_ratio`, `default_ratio`, `reduce_ratio`, `base_ratio`, `max_count`, `max_amount`) VALUES
(1, 0, 3, 5000, 1000, 50, 100, 20, 100000);