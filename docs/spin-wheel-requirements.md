# Spin Wheel (转盘提现) — Requirements Document

> **Version**: 1.0
> **Date**: 2026-06-16
> **Status**: Implemented (Go backend + H5 frontend)
> **Source**: Ported from C++ SpinHandler.cpp, redesigned with cleaner architecture

---

## 1. Overview

### 1.1 Feature Summary

Spin Wheel (转盘提现) is a user engagement feature combining a daily spin-to-earn mechanic with an invite-friend viral loop and a withdrawal system with automated risk control. Users spin a wheel daily to accumulate rewards through a controlled plot/script system, invite friends for bonus spins with a probability-based jackpot mechanic, and withdraw accumulated amounts to their game balance after passing a 4-rule automated audit.

### 1.2 Core Loop

```
Daily Free Spin → Accumulate Amount (via Plot Script)
       ↕
Invite Friends → Probability-based Jackpot Hit/Miss
       ↓
Reach Full Amount → Withdraw → Auto-Audit (4 Rules) → Manual Review → Credit Balance
```

### 1.3 Key Metrics

| Metric | Default |
|--------|---------|
| Probability base (RATIO_BASE) | 10,000 (= 100%) |
| Currency display divisor | 100 (internal cents → display dollars) |
| Default full_gold (target) | 100,000 (=$1,000.00 display) |
| Default round duration | 72 hours |
| Daily free spins | 1 |

---

## 2. Configuration System

### 2.1 Spin Config (`spin_config`)

Each row defines a complete spin wheel variant identified by `spin_id`. Multiple configs allow A/B testing and user segmentation.

| Field | Type | Description |
|-------|------|-------------|
| `spin_id` | VARCHAR(32) | Unique variant identifier |
| `full_gold` | BIGINT | Target amount to unlock withdrawal |
| `flow_multi` | INT | Flow requirement multiplier (÷ RATIO_BASE) |
| `time_limit_hour` | INT | Round duration in hours (default: 72) |
| `audit_usercnt` | INT | Rule 1: check last N invitees (-1=off) |
| `audit_rule_2_invitetotal_lt` | INT | Rule 2: invite count threshold |
| `audit_rule_2_flowmutil` | BIGINT | Rule 2: flow multiplier |
| `audit_rule_3_invtetotal_ge` | INT | Rule 3: invite count threshold |
| `audit_rule_4_users` | INT | Rule 4: suspect label count (-1=off) |
| `audit_rule_4_labels` | VARCHAR(256) | Rule 4: comma-separated suspect label IDs |
| `start_time` / `end_time` | BIGINT | Activity validity window (Unix timestamp) |
| `user_type` | TINYINT | 0=all, 1=tagged only, 2=specific UIDs |
| `tag_list` | VARCHAR(512) | Comma-separated label IDs for targeting |
| `user_list` | VARCHAR(512) | Comma-separated user IDs for targeting |
| `plot_list` | VARCHAR(256) | Comma-separated plot IDs |
| `invite_group_id` | INT | Links to `spin_invite_config` group |
| `priority` | INT | Higher = matched first |
| `box_gt` / `box_le` | INT | Gift box random range |
| `items_json` | TEXT | JSON array of wheel segments |
| `status` | TINYINT | 1=active, 0=disabled |

### 2.2 Plot Config (`spin_plot_config`)

Controls the deterministic amount progression per spin step.

| Field | Type | Description |
|-------|------|-------------|
| `step_inc` | INT | Fixed increment after all plot steps exhausted |
| `free_inc` | TEXT | JSON array of cumulative amounts, e.g. `[500, 800, 1200, ...]` |

**Logic**: At step N, if N < len(free_inc), the user's amount becomes free_inc[N]. After all steps, each spin adds `step_inc` to the current amount (capped at full_gold - 1). The plot is designed so the last step never reaches full_gold — free spins alone cannot complete the wheel.

### 2.3 Invite Config (`spin_invite_config`)

Probability settings per VIP level, grouped by `group_id`.

| Field | Type | Description |
|-------|------|-------------|
| `group_id` | INT | Config group (links to spin_config) |
| `vip` | INT | VIP level |
| `new_count` | INT | First N invites use `new_ratio` |
| `new_ratio` | INT | High hit rate for new inviters (÷10000 = %) |
| `default_ratio` | INT | Base hit rate after new_count |
| `reduce_ratio` | INT | Reduction per invite after new_count |
| `base_ratio` | INT | Minimum hit rate floor |
| `max_count` | INT | Guaranteed hit after this many invites |
| `max_amount` | BIGINT | Reserved |

**Hit probability calculation**:
```
if invite_count >= max_count → 100% hit
if level_invite <= new_count → hit_ratio = new_ratio
else → hit_ratio = max(default_ratio - reduce_ratio × (level_invite - new_count), base_ratio)
random(1, RATIO_BASE) <= hit_ratio → HIT (fill to full_gold)
```

### 2.4 Poster Config (`spin_poster_config`)

Multi-language sharing templates.

| Field | Type | Description |
|-------|------|-------------|
| `language` | VARCHAR(10) | Language code (en, pt, etc.) |
| `share_url` | VARCHAR(512) | Share URL template (`#code#` = invite code) |
| `telegram_url` | VARCHAR(512) | Telegram share link template |
| `whatsapp_url` | VARCHAR(512) | WhatsApp share link template |
| `share_url_prefix` | VARCHAR(256) | Share text prefix |
| `posters_json` | TEXT | JSON array of poster items |

### 2.5 Config Loading & Caching

- All configs loaded from DB into in-memory cache at service startup
- Protected by `sync.RWMutex` for thread safety
- `ReloadSpinConfigs()` available for hot-reload after admin changes
- No file-based configs — all database-driven for runtime configurability

---

## 3. User Features

### 3.1 Get Spin Info (`GET /activity/spin/info`)

**Purpose**: Main entry point when user opens the spin wheel page.

**Logic**:
1. Load or create `user_spin_data` (new users get initialized)
2. Match user to best spin config (priority, time range, user type)
3. Check round expiration → reset if expired
4. Grant daily free spin if new day
5. Return full page data: progress, items, boxes, records, invite code, countdown

**Response fields**: `result`, `tickets`, `amount`, `full_amount`, `end_time`, `items[]`, `boxes[]`, `rec_list[]`, `invite_code`, `cur_round`

### 3.2 Free Spin (`POST /activity/spin/do`)

**Purpose**: User's daily free spin to advance through the plot script.

**Logic**:
1. Acquire per-user Redis distributed lock (5s TTL, fail-fast on contention)
2. Validate: same-day limit (1/day), remaining tickets > 0, amount < full_gold
3. Consume ticket, record timestamp
4. Calculate next amount from plot script (see §2.2)
5. Match amount diff to wheel item position
6. Update user data, append spin record
7. Return: `result`, `tickets`, `amount`, `pos` (wheel position)

**Concurrency**: Redis `spin:lock:{userID}` prevents duplicate spins.

### 3.3 Invite Spin (`POST /activity/spin/invite-spin`)

**Purpose**: Triggered when an invited friend successfully registers.

**Request**: `{ "invite_uid": number }`

**Logic**:
1. Acquire lock (same as free spin)
2. Increment invite counters: `invite_count`, `total_invite`, `level_invite`
3. Check guaranteed hit (invite_count >= max_count) → fill to full_gold
4. Probability check (see §2.3 formula)
5. **Hit**: Amount → full_gold, record with invite_uid
6. **Miss**: Fall through to free spin logic (does NOT consume daily ticket)
7. Return: `result`, `tickets`, `amount`, `pos`, `is_hit`

### 3.4 Withdrawal (`POST /activity/spin/withdraw`)

**Pre-conditions**:
- User must have bound phone number
- `cur_amount >= full_gold`

**Logic**:
1. Validate phone binding
2. Calculate flow requirement: `flow = cur_amount × flow_multi / RATIO_BASE`
3. Create withdrawal order (unique order_no, status=pending)
4. Create order log entry
5. Reset user cycle data (amount=0, round_start_ts=0, plot_step=0, records=[])
6. **Asynchronously** run 4-rule auto-audit
7. Return: `result`, `order_id`

### 3.5 Withdrawal Log (`GET /activity/spin/withdraw-log`)

Paginated query of user's withdrawal orders. Parameters: `page` (default 1), `page_size` (default 20).

### 3.6 Current Data (`GET /activity/spin/cur-data`)

Lightweight endpoint for widgets/mini-components. Returns: `target`, `amount`, `tickets`.

### 3.7 Poster/Sharing (`GET /activity/spin/poster`)

Returns sharing configuration with invite code embedded in URLs. Supports `language` query parameter.

---

## 4. Auto-Audit System

Executed asynchronously after withdrawal submission. Order: **Rule 4 → Rule 2 → Rule 1 → Rule 3**.

### Rule 4 — Suspect Label Check
- **Trigger**: `audit_rule_4_users != -1`
- **Check**: Inviter and all invitees checked for suspect labels (from `audit_rule_4_labels`)
- **Result**: Any match → REJECT immediately
- **Skip to**: If no match, continue to Rule 2

### Rule 2 — Recharging User Flow Check
- **Trigger**: User has previous withdrawals OR has recharged
- **Check**: `total_recharge > 0` AND `total_invite < rule_2_invitetotal_lt` AND `valid_flow >= (rule_2_flowmutil / RATIO_BASE) × total_recharge / CURRENCY_BASE`
- **Result**: All conditions met → AUTO-APPROVE

### Rule 1 — Recent Invitee Recharge Check
- **Trigger**: `audit_usercnt > 0`
- **Check**: From round records, check last N unique invitees. If any has recharged → AUTO-APPROVE
- **Purpose**: Verify that invited users are real (they spent money)

### Rule 3 — Non-Recharger Subordinate Check
- **Trigger**: User has NOT recharged AND `total_invite >= rule_3_invtetotal_ge` (> 0)
- **Check**: All invitees have zero recharge
- **Result**: AUTO-REJECT (non-recharging user with all non-recharging invitees = fraud indicator)

### Default — Manual Review
If no rule matches → status set to DELAYED, queued for admin manual review.

### Audit Data Tracking
All audit decisions are recorded in `audit_json` with: `audit_rule_type`, `suspect_number`, `total_flow`, `toatal_recharge`, `invite_total`, `invite_total_recharge`.

---

## 5. Admin Management

All admin endpoints are under `/api/v1/admin/spin/` with JWT auth + operator role required.

### 5.1 Config CRUD
- `GET /configs` — List all spin configs (paginated)
- `GET /configs/:id` — Get single config detail
- `POST /configs` — Create new spin config
- `PUT /configs/:id` — Update existing config
- `DELETE /configs/:id` — Soft delete (status=0)

### 5.2 Plot Config CRUD
- `GET /plots` — List all plot configs
- `POST /plots` — Create new plot
- `DELETE /plots/:id` — Delete plot

### 5.3 Invite Config CRUD
- `GET /invites` — List invite configs (filterable by group_id)
- `POST /invites` — Create invite config
- `DELETE /invites/:id` — Delete invite config

### 5.4 Order Management
- `GET /orders` — List all withdrawal orders (filterable by status, paginated)
- `POST /orders/:id/audit` — Manual audit (approve/reject/delay)
- `GET /orders/:id/logs` — Get order audit trail

### 5.5 Statistics
- `GET /stats` — Aggregated statistics (total orders, pending, approved, rejected, total amount)

---

## 6. Data Model

### 6.1 Tables

| Table | Purpose |
|-------|---------|
| `spin_config` | Wheel variant configuration |
| `spin_plot_config` | Amount progression scripts |
| `spin_invite_config` | Invite probability per VIP/group |
| `spin_poster_config` | Multi-language sharing templates |
| `user_spin_data` | Per-user spin state (one row/user) |
| `spin_withdraw_order` | Withdrawal requests |
| `spin_order_log` | Order lifecycle audit trail |

### 6.2 Order Status Flow

```
PENDING (0) ──auto-audit──→ APPROVED (1) → credit balance
    │                           ↑
    ├──auto-reject──→ REJECTED (3)
    │
    └──no-rule-match──→ DELAYED (2) ──admin-approve──→ APPROVED (1)
                           │
                           └──admin-reject──→ REJECTED (3)
```

---

## 7. API Reference

### User Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/activity/spin/info` | Full spin page data |
| POST | `/api/v1/activity/spin/do` | Free spin |
| POST | `/api/v1/activity/spin/invite-spin` | Invite-triggered spin |
| POST | `/api/v1/activity/spin/withdraw` | Request withdrawal |
| GET | `/api/v1/activity/spin/withdraw-log` | Withdrawal history |
| GET | `/api/v1/activity/spin/cur-data` | Lightweight progress |
| GET | `/api/v1/activity/spin/poster` | Sharing config |

### Admin Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/admin/spin/configs` | List spin configs |
| GET | `/api/v1/admin/spin/configs/:id` | Get config detail |
| POST | `/api/v1/admin/spin/configs` | Create config |
| PUT | `/api/v1/admin/spin/configs/:id` | Update config |
| DELETE | `/api/v1/admin/spin/configs/:id` | Delete config |
| GET | `/api/v1/admin/spin/plots` | List plot configs |
| POST | `/api/v1/admin/spin/plots` | Create plot |
| DELETE | `/api/v1/admin/spin/plots/:id` | Delete plot |
| GET | `/api/v1/admin/spin/invites` | List invite configs |
| POST | `/api/v1/admin/spin/invites` | Create invite config |
| DELETE | `/api/v1/admin/spin/invites/:id` | Delete invite config |
| GET | `/api/v1/admin/spin/orders` | List orders |
| POST | `/api/v1/admin/spin/orders/:id/audit` | Audit order |
| GET | `/api/v1/admin/spin/orders/:id/logs` | Order logs |
| GET | `/api/v1/admin/spin/stats` | Dashboard stats |

---

## 8. Error Codes

| Code | Name | Description |
|------|------|-------------|
| 70001 | `ErrSpinDayLimit` | Daily free spin limit reached |
| 70002 | `ErrNoChance` | No remaining spin chances |
| 70003 | `ErrSpinAmountFull` | Amount already at/above target |
| 70004 | `ErrSpinBindPhone` | Phone number not bound |
| 70005 | `ErrSpinUserDataErr` | User spin data error |
| 70006 | `ErrSpinNotActive` | No active spin configuration |
| 70007 | `ErrSpinOrderNotFound` | Withdrawal order not found |
| 70008 | `ErrSpinOrderPending` | Already have a pending order |

---

## 9. Frontend Implementation

### 9.1 Page Route

`/spin` — Full-page spin wheel experience with 3 tabs.

### 9.2 Tabs

| Tab | Content |
|-----|---------|
| **Wheel** | SVG wheel, progress bar, stats cards, gift boxes, spin history, withdraw button |
| **Records** | Paginated withdrawal order list with status badges |
| **Invite** | Invite code (copy), how-it-works steps, share links (Telegram, WhatsApp) |

### 9.3 Key UX Decisions

- **SVG wheel** instead of CSS conic-gradient for crisp rendering at all sizes
- **Graceful degradation**: Falls back to demo data when API is unavailable (with DemoBadge)
- **Amount display**: Internal units ÷ 100 = display dollars (e.g., 50000 → $500.00)
- **Spin animation**: 4-second cubic-bezier deceleration (6 full rotations + target offset)
- **Toast notifications**: Inline toast for all user actions (no alert/prompt)
- **Dark theme**: Consistent with project's dark gold/crimson color scheme

### 9.4 Component Structure

```
src/app/spin/page.tsx     — Main page (self-contained, ~640 lines)
src/services/activity.ts  — API functions + TypeScript types (already existed)
```

---

## 10. Architecture Decisions (vs C++ Original)

| Aspect | C++ Original | Go Implementation |
|--------|-------------|-------------------|
| Config storage | JSON files + Redis labels | Database tables + in-memory cache |
| Config loading | Startup file read | DB query + `sync.RWMutex` cache |
| Concurrency | Process-level mutex | Redis distributed lock per user |
| Transactions | Manual commit/rollback | GORM `db.Transaction()` |
| Audit | Synchronous in handler | Asynchronous goroutine |
| User targeting | RPC to label service | Simplified (extensible via tag_list) |
| Code organization | 2000-line monolith | Separate handler/admin/model files |
| Error handling | Integer codes | `BizError` typed errors with HTTP mapping |