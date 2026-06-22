-- ============================================================================
-- RockGame Sharding Helper
-- Generates all 16 hash-sharded tables (user_id % 16)
-- Tables that need sharding are listed in @sharded_tables
-- ============================================================================

DELIMITER $$

DROP PROCEDURE IF EXISTS `create_sharded_tables`$$

CREATE PROCEDURE `create_sharded_tables`()
BEGIN
    DECLARE i INT DEFAULT 0;
    DECLARE shard_sql TEXT;
    DECLARE table_suffix VARCHAR(4);

    -- Hash sharded tables (by user_id % 16)
    -- Format: CREATE TABLE `{table}_{XX}` LIKE `{table}`;
    WHILE i < 16 DO
        SET table_suffix = LPAD(i, 2, '0');

        -- user_referral
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `user_referral_', table_suffix, '` LIKE `user_referral`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- user_inventory
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `user_inventory_', table_suffix, '` LIKE `user_inventory`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- item_log
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `item_log_', table_suffix, '` LIKE `item_log`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- recharge_order
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `recharge_order_', table_suffix, '` LIKE `recharge_order`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- withdraw_order
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `withdraw_order_', table_suffix, '` LIKE `withdraw_order`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- user_recharge_bonus
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `user_recharge_bonus_', table_suffix, '` LIKE `user_recharge_bonus`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- user_vip
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `user_vip_', table_suffix, '` LIKE `user_vip`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- vip_benefit_log
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `vip_benefit_log_', table_suffix, '` LIKE `vip_benefit_log`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- user_task
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `user_task_', table_suffix, '` LIKE `user_task`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- user_activity
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `user_activity_', table_suffix, '` LIKE `user_activity`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- activity_records
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `activity_records_', table_suffix, '` LIKE `activity_records`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- mail
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `mail_', table_suffix, '` LIKE `mail`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- game_transaction
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `game_transaction_', table_suffix, '` LIKE `game_transaction`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- agent_commission
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `agent_commission_', table_suffix, '` LIKE `agent_commission`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- user_tag
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `user_tag_', table_suffix, '` LIKE `user_tag`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- user_red_dot
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `user_red_dot_', table_suffix, '` LIKE `user_red_dot`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        SET i = i + 1;
    END WHILE;

    SELECT CONCAT('Created ', i, ' x 16 = ', i * 16, ' sharded tables') AS result;
END$$

DELIMITER ;

-- Execute: CALL create_sharded_tables();

-- Daily partitioned tables (create_daily_tables, cleanup_old_log_tables)
-- moved to rockgame_log database (sql/sharding_log.sql)
