-- ============================================================================
-- RockGame Seed Data
-- Insert initial configuration data for development
-- ============================================================================

USE `rockgame`;

-- ============================================================================
-- Admin User
-- IMPORTANT: The default password below is ONLY for local development.
-- In production, create admin accounts via the admin panel or a secure script.
-- Never deploy the default admin/admin123 credentials to staging or production.
-- ============================================================================
INSERT INTO `admin_user` (`username`, `password_hash`, `role`, `status`) VALUES
('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'super_admin', 1);

-- ============================================================================
-- VIP Level Config
-- ============================================================================
INSERT INTO `vip_level_config` (`level`, `name`, `growth_required`, `withdraw_fee_rate`, `daily_signin_bonus`, `benefits_json`, `status`) VALUES
(1,  'VIP1 - Bronze',    0,       2.00, 1.0, '{"max_withdraw_daily": 500}',                    1),
(2,  'VIP2 - Silver',    1000,    1.80, 1.2, '{"max_withdraw_daily": 1000}',                   1),
(3,  'VIP3 - Gold',      5000,    1.50, 1.5, '{"max_withdraw_daily": 3000, "birthday_bonus": 50}', 1),
(4,  'VIP4 - Platinum',   20000,   1.20, 2.0, '{"max_withdraw_daily": 5000, "birthday_bonus": 100}', 1),
(5,  'VIP5 - Diamond',    50000,   0.80, 2.5, '{"max_withdraw_daily": 10000, "birthday_bonus": 200, "exclusive_activity": true}', 1),
(6,  'VIP6 - Master',    100000,  0.50, 3.0, '{"max_withdraw_daily": 20000, "exclusive_activity": true}', 1),
(7,  'VIP7 - Grand',     200000,  0.30, 3.5, '{"max_withdraw_daily": 50000, "exclusive_activity": true, "exclusive_support": true}', 1),
(8,  'VIP8 - Legend',    500000,  0.10, 4.0, '{"max_withdraw_daily": 100000, "exclusive_activity": true, "exclusive_support": true}', 1),
(9,  'VIP9 - Mythic',    1000000, 0.05, 5.0, '{"max_withdraw_daily": 200000, "exclusive_activity": true, "exclusive_support": true}', 1),
(10, 'VIP10 - God',      5000000, 0.00, 10.0, '{"max_withdraw_daily": 999999, "exclusive_activity": true, "exclusive_support": true, "all_benefits": true}', 1);

-- ============================================================================
-- Red Dot Config
-- ============================================================================
INSERT INTO `red_dot_config` (`path`, `parent_path`, `display_name`, `i18n_key`) VALUES
('mail',      '',                  'Mail',      'red_dot.mail'),
('activity',  '',                  'Activity',  'red_dot.activity'),
('task',      '',                  'Task',      'red_dot.task'),
('mail.unread',      'mail',        'Unread',    'red_dot.mail.unread'),
('mail.attachment',  'mail',        'Attachment','red_dot.mail.attachment'),
('activity.signin',    'activity',   'Signin',    'red_dot.activity.signin'),
('activity.wheel',     'activity',   'Wheel',     'red_dot.activity.wheel'),
('activity.recharge',  'activity',   'Recharge',  'red_dot.activity.recharge'),
('activity.gift',      'activity',   'Gift',      'red_dot.activity.gift'),
('task.daily',      'task',          'Daily',     'red_dot.task.daily'),
('task.weekly',     'task',          'Weekly',    'red_dot.task.weekly'),
('task.growth',     'task',          'Growth',    'red_dot.task.growth');

-- ============================================================================
-- Event Definitions
-- ============================================================================
INSERT INTO `event_define` (`event_name`, `category`, `description`) VALUES
('user_register',     'user',     'User registered'),
('user_login',        'user',     'User logged in'),
('user_logout',       'user',     'User logged out'),
('recharge_created',  'payment',  'Recharge order created'),
('recharge_success',  'payment',  'Recharge success'),
('recharge_failed',   'payment',  'Recharge failed'),
('withdraw_created',  'payment',  'Withdraw order created'),
('withdraw_success',  'payment',  'Withdraw success'),
('withdraw_rejected', 'payment',  'Withdraw rejected'),
('game_launch',       'game',     'Game launched'),
('game_bet',          'game',     'Game bet placed'),
('game_settle',       'game',     'Game settled'),
('game_cancel',       'game',     'Game cancelled'),
('activity_join',     'activity', 'Activity joined'),
('activity_draw',     'activity', 'Activity drawn'),
('activity_claim',    'activity', 'Activity reward claimed'),
('task_complete',      'activity', 'Task completed'),
('task_claim',         'activity', 'Task reward claimed'),
('mail_send',          'user',     'Mail sent'),
('vip_upgrade',        'user',     'VIP level upgraded');

-- ============================================================================
-- Payment Channels (dev stubs)
-- ============================================================================
INSERT INTO `payment_channel` (`name`, `type`, `config_json`, `status`, `supported_regions`, `min_amount`, `max_amount`, `sort_order`) VALUES
('Stripe',    'stripe', '{"api_key": "pk_test_xxx", "secret_key": "sk_test_xxx"}', 1, 'US,CA,EU,UK,AU', 10.0000, 10000.0000, 1),
('PayPal',    'paypal', '{"client_id": "xxx", "client_secret": "xxx"}',   1, 'US,CA,EU,UK',     10.0000, 5000.0000,  2),
('USDT-TRC20', 'usdt', '{"wallet_address": "Txxxxxxxxxxx"}',              0, 'ALL',              50.0000, 50000.0000, 3);

-- ============================================================================
-- Currency Rates
-- ============================================================================
INSERT INTO `currency_rate` (`from_currency`, `to_currency`, `rate`, `source`) VALUES
('EUR', 'USD',  1.08500000, 'ecb'),
('GBP', 'USD',  1.27000000, 'ecb'),
('JPY', 'USD',  0.00670000, 'ecb'),
('USDT','USD',  1.00000000, 'fixed'),
('BRL', 'USD',  0.18000000, 'ecb'),
('INR', 'USD',  0.01200000, 'ecb');

-- ============================================================================
-- Lobby Config
-- ============================================================================
INSERT INTO `lobby_config` (`lobby_name`, `parent_id`, `sort_order`, `status`, `language`, `icon`) VALUES
('Main Hall',     0, 1, 1, 'en', '/icons/hall_main.png'),
('Slots',         0, 2, 1, 'en', '/icons/hall_slots.png'),
('Live Casino',   0, 3, 1, 'en', '/icons/hall_live.png'),
('Sports',         0, 4, 1, 'en', '/icons/hall_sports.png'),
('Fishing',        0, 5, 1, 'en', '/icons/hall_fishing.png'),
('Table Games',    0, 6, 1, 'en', '/icons/hall_table.png');

-- ============================================================================
-- Game Categories
-- ============================================================================
INSERT INTO `game_category` (`lobby_id`, `name`, `icon`, `sort_order`, `status`) VALUES
(1, 'Popular',      '/icons/cat_hot.png',     1, 1),
(1, 'New Games',    '/icons/cat_new.png',     2, 1),
(1, 'Recommended', '/icons/cat_rec.png',     3, 1),
(2, 'Classic Slots',  '/icons/cat_classic.png', 1, 1),
(2, 'Video Slots',    '/icons/cat_video.png',   2, 1),
(2, 'Megaways',       '/icons/cat_mega.png',    3, 1),
(3, 'Baccarat',     '/icons/cat_bac.png',     1, 1),
(3, 'Blackjack',    '/icons/cat_bj.png',      2, 1),
(3, 'Roulette',     '/icons/cat_roulette.png',3, 1);

-- ============================================================================
-- Task Define (daily tasks)
-- ============================================================================
INSERT INTO `task_define` (`name`, `type`, `cycle`, `target_key`, `target_value`, `reward_type`, `reward_value`, `sort_order`, `status`) VALUES
('Daily Login',       'daily',  1, 'login',         1,   'coin',  '{"amount": 100}',  1, 1),
('Play 3 Games',      'daily',  1, 'game_launch',   3,   'coin',  '{"amount": 200}',  2, 1),
('Daily Bet 1000',    'daily',  1, 'total_bet',     1000,'coin',  '{"amount": 500}',  3, 1),
('First Recharge',     'growth', 0, 'first_recharge',1,   'coin',  '{"amount": 5000}', 4, 1),
('Accumulate VIP3',   'growth', 0, 'vip_level',     3,   'item',  '{"item_id": 1, "qty": 1}', 5, 1),
('Weekly Active 5',   'weekly', 7, 'login_days',    5,   'coin',  '{"amount": 2000}', 6, 1);

-- ============================================================================
-- Item Define (sample items)
-- ============================================================================
INSERT INTO `item_define` (`name`, `icon`, `type`, `duration`, `stackable`, `description`, `i18n_key`, `status`) VALUES
('Gold 100',        '/items/coin100.png',      1, 0, 1, '100 Gold Coins',                'item.gold_100',   1),
('Gold 500',        '/items/coin500.png',      1, 0, 1, '500 Gold Coins',                'item.gold_500',   1),
('Gold 1000',       '/items/coin1000.png',     1, 0, 1, '1000 Gold Coins',               'item.gold_1000',  1),
('Free Spin Ticket','/items/spin_ticket.png',   1, 0, 1, '1 Free Wheel Spin',             'item.free_spin',  1),
('VIP Trial Card',  '/items/vip_trial.png',     2, 86400, 0, '24h VIP Experience Card',     'item.vip_trial',  1),
('Double EXP Card', '/items/exp_double.png',    2, 3600, 0, '1h Double Experience',        'item.exp_double', 1),
('Avatar Frame',    '/items/frame_gold.png',    3, 0, 0, 'Golden Avatar Frame',         'item.frame_gold', 1),
('Title: Champion', '/items/title_champ.png',   3, 0, 0, 'Champion Title',               'item.title_champ',1);

-- ============================================================================
-- Banner (sample)
-- ============================================================================
INSERT INTO `banner` (`lobby_id`, `image_url`, `link_url`, `weight`, `start_time`, `end_time`, `target_lang`, `status`) VALUES
(0, 'https://cdn.rockgame.com/banner/welcome_en.jpg', '/activity/welcome-bonus', 100, '2026-01-01 00:00:00', '2026-12-31 23:59:59', '', 1),
(0, 'https://cdn.rockgame.com/banner/vip_promo_en.jpg', '/vip', 80, '2026-01-01 00:00:00', '2026-12-31 23:59:59', '', 1),
(0, 'https://cdn.rockgame.com/banner/first_recharge_en.jpg', '/shop', 60, '2026-06-01 00:00:00', '2026-06-30 23:59:59', 'en', 1);

-- ============================================================================
-- Splash Popup (sample)
-- ============================================================================
INSERT INTO `splash_popup` (`title`, `content`, `image_url`, `link_url`, `trigger_rules`, `daily_limit`, `priority`, `status`) VALUES
('Welcome Bonus!', 'Get 100% on your first recharge up to $500', 'https://cdn.rockgame.com/popup/welcome.png', '/shop', '{"new_users_only": true}', 1, 100, 1);

-- ============================================================================
-- Game Vendor (dev stubs)
-- ============================================================================
INSERT INTO `game_vendor` (`name`, `api_key`, `api_secret`, `callback_url`, `status`, `timeout_config`, `retry_policy`, `error_threshold`) VALUES
('PG Soft',   'pk_pg_test', 'sk_pg_test', 'http://localhost:8101/api/v1/wallet/pg/callback',   1, '{"connect": 3, "read": 10}', '{"max_retry": 3, "retryable": ["balance", "bet"]}', 30),
('Pragmatic', 'pk_pp_test', 'sk_pp_test', 'http://localhost:8101/api/v1/wallet/pp/callback',   1, '{"connect": 3, "read": 10}', '{"max_retry": 3, "retryable": ["balance", "bet"]}', 30),
('Evolution', 'pk_evo_test','sk_evo_test','http://localhost:8101/api/v1/wallet/evo/callback',  1, '{"connect": 5, "read": 15}', '{"max_retry": 2, "retryable": ["balance"]}', 20);

-- ============================================================================
-- Rank Define (sample)
-- ============================================================================
INSERT INTO `rank_define` (`name`, `type`, `period`, `reset_rule`, `reward_config`, `status`) VALUES
('Daily Recharge',  'recharge', 'daily',   '0 0 * * *',  '{"top1": {"coin": 10000}, "top3": {"coin": 5000}, "top10": {"coin": 1000}}', 1),
('Weekly Win',      'win',     'weekly',  '0 0 * * 1',  '{"top1": {"coin": 50000}, "top3": {"coin": 20000}, "top10": {"coin": 5000}}', 1),
('Monthly Agent',   'agent',   'monthly', '0 0 1 * *',  '{"top1": {"coin": 100000}, "top3": {"coin": 50000}, "top10": {"coin": 10000}}', 1);

-- ============================================================================
-- Activity: Daily Check-in (每日签到)
-- 7-day cycle, rewards increase each day, Day 7 is special.
-- ============================================================================
INSERT INTO `activity_define` (`name`, `type`, `handler_name`, `config_json`, `start_time`, `end_time`, `status`, `priority`) VALUES
('Daily Check-in', 'signin', 'checkin',
 '{
    "cycle_days": 7,
    "reset_on_miss": true,
    "rewards": [
        {"day": 1, "reward_type": "bonus", "reward_value": 1.00},
        {"day": 2, "reward_type": "bonus", "reward_value": 2.00},
        {"day": 3, "reward_type": "bonus", "reward_value": 3.00},
        {"day": 4, "reward_type": "bonus", "reward_value": 5.00},
        {"day": 5, "reward_type": "bonus", "reward_value": 8.00},
        {"day": 6, "reward_type": "bonus", "reward_value": 12.00},
        {"day": 7, "reward_type": "bonus", "reward_value": 20.00}
    ]
 }',
 '2026-01-01 00:00:00', '2030-12-31 23:59:59', 1, 100);

-- ============================================================================
-- Activity: Lucky Wheel (转盘抽奖)
-- 8 prize slots with weighted probabilities, 3 free spins/day, $5 per paid spin.
-- ============================================================================
INSERT INTO `activity_define` (`name`, `type`, `handler_name`, `config_json`, `start_time`, `end_time`, `status`, `priority`) VALUES
('Lucky Wheel', 'wheel', 'spin_wheel',
 '{
    "free_spins_per_day": 3,
    "spin_cost": 5.00,
    "spin_cost_type": "bonus",
    "cooldown_sec": 5,
    "max_spins_per_day": 50,
    "prizes": [
        {"id": 1,  "name": "$0.50 Bonus",     "type": "bonus", "value": 0.50,  "item_id": 0, "weight": 300, "icon": "/wheel/prize_050.png",     "rarity": "common",    "stock": -1},
        {"id": 2,  "name": "$1.00 Bonus",     "type": "bonus", "value": 1.00,  "item_id": 0, "weight": 250, "icon": "/wheel/prize_100.png",     "rarity": "common",    "stock": -1},
        {"id": 3,  "name": "$2.00 Bonus",     "type": "bonus", "value": 2.00,  "item_id": 0, "weight": 150, "icon": "/wheel/prize_200.png",     "rarity": "common",    "stock": -1},
        {"id": 4,  "name": "$5.00 Bonus",     "type": "bonus", "value": 5.00,  "item_id": 0, "weight": 100, "icon": "/wheel/prize_500.png",     "rarity": "rare",      "stock": -1},
        {"id": 5,  "name": "$10.00 Bonus",    "type": "bonus", "value": 10.00, "item_id": 0, "weight": 60,  "icon": "/wheel/prize_1000.png",    "rarity": "rare",      "stock": -1},
        {"id": 6,  "name": "100 Coins",       "type": "coin",  "value": 100,   "item_id": 0, "weight": 80,  "icon": "/wheel/prize_coin100.png", "rarity": "common",    "stock": -1},
        {"id": 7,  "name": "Free Spin Ticket","type": "item",  "value": 1.00,  "item_id": 4, "weight": 40,  "icon": "/wheel/prize_spin.png",    "rarity": "epic",      "stock": 500},
        {"id": 8,  "name": "$50.00 Jackpot",  "type": "bonus", "value": 50.00, "item_id": 0, "weight": 5,   "icon": "/wheel/prize_jackpot.png",  "rarity": "legendary", "stock": 10},
        {"id": 9,  "name": "Thanks",          "type": "empty", "value": 0,     "item_id": 0, "weight": 15,  "icon": "/wheel/prize_empty.png",   "rarity": "common",    "stock": -1}
    ]
 }',
 '2026-01-01 00:00:00', '2030-12-31 23:59:59', 1, 90);

-- Wheel prize inventory (for limited prizes)
INSERT INTO `activity_inventory` (`activity_id`, `reward_id`, `total`, `remaining`)
SELECT `id`, 7, 500, 500 FROM `activity_define` WHERE `type` = 'wheel' AND `handler_name` = 'spin_wheel' LIMIT 1;

INSERT INTO `activity_inventory` (`activity_id`, `reward_id`, `total`, `remaining`)
SELECT `id`, 8, 10, 10 FROM `activity_define` WHERE `type` = 'wheel' AND `handler_name` = 'spin_wheel' LIMIT 1;
