-- Migration: vendor add logo, category add parent_id + updated_at, game_info add rtp/bet fields
-- Date: 2026-06-08

-- game_vendor: add logo column
ALTER TABLE `game_vendor` ADD COLUMN `logo` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '厂商Logo' AFTER `name`;

-- game_category: add parent_id for two-level hierarchy + updated_at
ALTER TABLE `game_category`
    ADD COLUMN `parent_id` BIGINT NOT NULL DEFAULT 0 COMMENT '父分类ID(0=顶级)' AFTER `lobby_id`,
    ADD COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER `created_at`,
    ADD KEY `idx_parent` (`parent_id`);

-- game_info: add rtp, bet_min, bet_max columns (used by admin CRUD)
ALTER TABLE `game_info`
    ADD COLUMN `rtp` DECIMAL(5,2) NOT NULL DEFAULT 0 COMMENT 'RTP(%)' AFTER `category_id`,
    ADD COLUMN `bet_min` DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '最小下注' AFTER `rtp`,
    ADD COLUMN `bet_max` DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '最大下注' AFTER `bet_min`;
