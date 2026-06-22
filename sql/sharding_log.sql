-- ============================================================================
-- RockGame Log Database - Daily Partitioned Tables
-- Generates daily tables for event_log and click_log
-- ============================================================================

DELIMITER $$

DROP PROCEDURE IF EXISTS `create_daily_log_tables`$$

CREATE PROCEDURE `create_daily_log_tables`(IN days INT)
BEGIN
    DECLARE i INT DEFAULT 0;
    DECLARE date_str VARCHAR(8);
    DECLARE shard_sql TEXT;

    WHILE i < days DO
        SET date_str = DATE_FORMAT(DATE_ADD(CURDATE(), INTERVAL i DAY), '%Y%m%d');

        -- event_log_{YYYYMMDD}
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `event_log_', date_str, '` LIKE `event_log`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        -- click_log_{YYYYMMDD}
        SET shard_sql = CONCAT('CREATE TABLE IF NOT EXISTS `click_log_', date_str, '` LIKE `click_log`');
        SET @sql = shard_sql; PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

        SET i = i + 1;
    END WHILE;

    SELECT CONCAT('Created daily log tables for next ', days, ' days (2 tables/day)') AS result;
END$$

DELIMITER ;

-- Execute: CALL create_daily_log_tables(7);

-- ============================================================================
-- Cleanup: Drop old daily log tables beyond retention days
-- Run daily via cron to keep table count manageable
-- ============================================================================

DELIMITER $$

DROP PROCEDURE IF EXISTS `cleanup_old_log_tables`$$

CREATE PROCEDURE `cleanup_old_log_tables`(IN retention_days INT)
BEGIN
    DECLARE cutoff_date VARCHAR(8);
    DECLARE done INT DEFAULT 0;
    DECLARE tbl_name VARCHAR(64);
    DECLARE cur CURSOR FOR
        SELECT table_name FROM information_schema.tables
        WHERE table_schema = DATABASE()
          AND (table_name LIKE 'event_log_%' OR table_name LIKE 'click_log_%')
          AND table_name REGEXP '[0-9]{8}$'
          AND SUBSTRING(table_name, -8) < cutoff_date;
    DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;

    SET cutoff_date = DATE_FORMAT(DATE_SUB(CURDATE(), INTERVAL retention_days DAY), '%Y%m%d');

    OPEN cur;
    read_loop: LOOP
        FETCH cur INTO tbl_name;
        IF done THEN LEAVE read_loop; END IF;
        SET @drop_sql = CONCAT('DROP TABLE IF EXISTS `', tbl_name, '`');
        PREPARE stmt FROM @drop_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    END LOOP;
    CLOSE cur;

    SELECT CONCAT('Cleaned up daily log tables older than ', retention_days, ' days (before ', cutoff_date, ')') AS result;
END$$

DELIMITER ;

-- Execute: CALL cleanup_old_log_tables(90);

-- ============================================================================
-- Scheduled event: auto-create + cleanup (run via MySQL event scheduler)
-- Enable once: SET GLOBAL event_scheduler = ON;
-- ============================================================================

-- CREATE EVENT IF NOT EXISTS `evt_maintain_log_tables`
-- ON SCHEDULE EVERY 1 DAY STARTS CURDATE() + INTERVAL 1 HOUR
-- DO
-- BEGIN
--     CALL create_daily_log_tables(8);
--     CALL cleanup_old_log_tables(90);
-- END;
