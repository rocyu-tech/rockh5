-- Migration: Add missing columns to admin_user table
-- The original schema.sql only had: id, username, password_hash, role, status, last_login, created_at, updated_at
-- This migration adds the columns expected by the admin CRUD handlers.

USE `rockgame`;

ALTER TABLE `admin_user`
  ADD COLUMN IF NOT EXISTS `real_name`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT 'Display name' AFTER `password_hash`,
  ADD COLUMN IF NOT EXISTS `email`         VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Email' AFTER `real_name`,
  ADD COLUMN IF NOT EXISTS `last_login_ip` VARCHAR(45)  NOT NULL DEFAULT '' COMMENT 'Last login IP' AFTER `last_login`,
  CHANGE COLUMN  `last_login` `last_login_at` DATETIME NULL COMMENT 'Last login time';