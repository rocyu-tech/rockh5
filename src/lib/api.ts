import axios, { AxiosError, InternalAxiosRequestConfig } from "axios";
// P0-8 FIX: import the auth store to synchronize zustand state on forceLogout.
// This creates a circular import (store/auth.ts imports TOKEN_KEY from this file),
// but it is safe because:
//   - Both modules only reference each other inside function bodies (runtime),
//     never at module top-level (load time).
//   - ESM live bindings resolve correctly once both modules finish loading,
//     which happens before any user interaction triggers forceLogout.
import { useAuthStore } from "@/store/auth";

const TOKEN_KEY = "rockgame_token";
const REFRESH_TOKEN_KEY = "rockgame_refresh_token";

export const api = axios.create({
  baseURL: "/api/v1",
  timeout: 15000,
  withCredentials: true, // P0-6: send httpOnly cookies (set by /auth/login) on every request
  headers: {
    "Content-Type": "application/json",
  },
});

// Request interceptor - add auth token
api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    if (typeof window !== "undefined") {
      const token = localStorage.getItem(TOKEN_KEY);
      if (token && config.headers) {
        config.headers.Authorization = `Bearer ${token}`;
      }
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor - handle token refresh
let isRefreshing = false;
let failedQueue: Array<{
  resolve: (value: unknown) => void;
  reject: (reason?: unknown) => void;
}> = [];

function processQueue(error: unknown, token: string | null = null) {
  failedQueue.forEach((prom) => {
    if (error) {
      prom.reject(error);
    } else {
      prom.resolve(token);
    }
  });
  failedQueue = [];
}

// Helper: clear tokens and trigger login dialog
//
// P0-8 FIX: previously this function only cleared localStorage and dispatched
// the 'auth:logout' event, leaving the zustand store's `isLoggedIn` flag
// still `true`. AppProvider.handleAuthLogout checks `if (!currentlyLoggedIn)`
// before opening the login modal — so when a 401 triggered forceLogout, the
// modal never appeared and the user was stuck in a "looks-logged-in but every
// request fails" limbo state.
//
// Fix: synchronously clear zustand state (no async logout API call) so
// isLoggedIn=false BEFORE the event fires. AppProvider's guard passes and
// the login modal opens correctly.
function forceLogout() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
  // Synchronously clear zustand auth state (no API call, no await)
  useAuthStore.setState({
    token: null,
    refreshToken: null,
    user: null,
    assets: null,
    isLoggedIn: false,
    lastError: null,
  });
  window.dispatchEvent(new CustomEvent("auth:logout"));
}

// Debounce: only trigger login dialog once even if multiple 401s fire
let lastLogoutTime = 0;
const LOGOUT_COOLDOWN = 5000; // 5 seconds cooldown between login popups

api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as InternalAxiosRequestConfig & {
      _retry?: boolean;
      _noAuth?: boolean;
    };

    // If this request was intentionally sent without auth, just reject
    if (originalRequest._noAuth) {
      return Promise.reject(error);
    }

    const status = error.response?.status;
    const data = error.response?.data as { code?: number };

    const isAuthError =
      status === 401 ||
      data?.code === 10003 ||
      data?.code === 20005 ||
      data?.code === 20006;

    if (!isAuthError || originalRequest._retry) {
      return Promise.reject(error);
    }

    // Check if we have a refresh token at all
    const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY);

    if (!refreshToken) {
      // No refresh token — user is not logged in or session expired
      // Only trigger login popup once per 5 seconds to avoid spam
      const now = Date.now();
      if (now - lastLogoutTime > LOGOUT_COOLDOWN) {
        lastLogoutTime = now;
        forceLogout();
      }
      return Promise.reject(error);
    }

    // We have a refresh token, try to refresh
    if (isRefreshing) {
      return new Promise((resolve, reject) => {
        failedQueue.push({ resolve, reject });
      }).then((token) => {
        if (originalRequest.headers) {
          originalRequest.headers.Authorization = `Bearer ${token}`;
        }
        return api(originalRequest);
      });
    }

    originalRequest._retry = true;
    isRefreshing = true;

    try {
      const res = await axios.post("/api/v1/auth/refresh", {
        refresh_token: refreshToken,
      });

      const newToken = res.data?.access_token;
      const newRefreshToken = res.data?.refresh_token;

      if (newToken) {
        localStorage.setItem(TOKEN_KEY, newToken);
        if (newRefreshToken) {
          localStorage.setItem(REFRESH_TOKEN_KEY, newRefreshToken);
        }
        if (originalRequest.headers) {
          originalRequest.headers.Authorization = `Bearer ${newToken}`;
        }
        processQueue(null, newToken);
        // P0-7: notify active WebSocket clients to refresh their token.
        // Browsers can't set Authorization headers on WS upgrade, so the WS
        // client holds a stale token after access_token rotation. This event
        // lets GameWSClient instances re-connect with the fresh token before
        // the old one expires (15min TTL) — preventing mid-hand disconnects.
        if (typeof window !== 'undefined') {
          window.dispatchEvent(new CustomEvent('auth:token-refreshed', { detail: { token: newToken } }));
        }
        return api(originalRequest);
      }

      // Refresh returned no token — force logout
      forceLogout();
      processQueue(error, null);
      return Promise.reject(error);
    } catch (refreshError) {
      // Refresh failed — force logout
      forceLogout();
      processQueue(refreshError, null);
      return Promise.reject(refreshError);
    } finally {
      isRefreshing = false;
    }
  }
);

// === Types ===

export interface Banner {
  id: number;
  title: string;
  image_url: string;
  link_url?: string;
  sort_order: number;
  status: number;
}

export interface Category {
  id: number;
  name: string;
  icon?: string;
  sort_order: number;
  game_count?: number;
}

export interface GameVendor {
  id: number;
  name: string;
  logo?: string;
}

export interface Game {
  id: number;
  name: string;
  cover?: string;
  vendor_id: number;
  vendor_name?: string;
  game_type?: string; // P0: "vendor" | "self"
  launch_url?: string; // P0: for self-developed games
  category_id: number;
  category_name?: string;
  status: number;
  hot?: boolean;
  new?: boolean;
  tag?: string;
  is_favorite?: boolean;
}

export interface GameListResponse {
  list: Game[];
  total: number;
  page: number;
  page_size: number;
}

export interface UserProfile {
  id: number;
  email: string;
  phone?: string;
  nickname?: string;
  avatar?: string;
  vip_level: number;
  created_at: string;
}

export interface UserAssets {
  balance: number;
  cash_balance: number;
  bonus_balance: number;
  frozen_balance: number;
  total: number;
  total_recharge?: number;
  total_withdraw?: number;
  total_bet?: number;
  total_win?: number;
  currency: string;
}

export interface VIPLevel {
  level: number;
  name: string;
  growth_required: number;
  benefits: string[];
  icon?: string;
  withdraw_fee_rate?: number;
  daily_signin_bonus?: number;
}

export interface Activity {
  id: number;
  title: string;
  description?: string;
  image_url?: string;
  link_url?: string;
  start_time?: string;
  end_time?: string;
  status: number;
}

// === API Functions ===

// Auth
export const authApi = {
  login: (email: string, password: string) =>
    api.post<{ access_token: string; refresh_token: string }>("/auth/login", { email, password }),

  register: (data: { email: string; password: string; confirm_password: string; phone?: string }) =>
    api.post<{ access_token: string; refresh_token: string }>("/auth/register", data),

  refresh: (refreshToken: string) =>
    api.post<{ access_token: string; refresh_token: string }>("/auth/refresh", { refresh_token: refreshToken }),

  logout: () => api.post<{ result: string }>("/auth/logout"),

  // P1: password reset — 2-step flow.
  //   1. requestPasswordReset(email) → backend generates a 64-char hex token,
  //      stores it in Redis with 15min TTL, and (in dev) logs the token.
  //      In prod with SMTP wired, it emails a reset link.
  //   2. confirmPasswordReset(token, new_password) → validates token, sets
  //      new password, deletes token, force-logs-out all existing sessions.
  requestPasswordReset: (email: string) =>
    api.post<{ message: string }>("/auth/password-reset/request", { email }, { _noAuth: true } as object),
  confirmPasswordReset: (token: string, newPassword: string) =>
    api.post<{ message: string }>("/auth/password-reset/confirm", { token, new_password: newPassword }, { _noAuth: true } as object),
};

// VIP — P1
export const vipApi = {
  // Returns translated levels. Pass `lang` to get localized benefit names.
  // The VIPLevel interface (above) is the canonical shape used by existing
  // components; we keep the response typed as VIPLevel[] for compatibility.
  getLevels: (lang?: string) =>
    api.get<VIPLevel[]>("/vip/levels", { params: lang ? { lang } : undefined }),
  getInfo: () =>
    api.get<{
      level: number;
      growth: number;
      progress: number;
      next_level?: { level: number; name: string; growth_required: number } | Record<string, never>;
    }>("/vip/info"),
  upgrade: () => api.post<{ current_level: number; upgraded: boolean; old_level: number; new_level: number }>("/vip/upgrade", {}),
};

// Game history — P1
export const historyApi = {
  list: (params: { type?: 'all' | 'slot' | 'poker' | 'baccarat' | 'dragon'; page?: number; page_size?: number } = {}) =>
    api.get<{
      list: Array<{
        id: number;
        game_type: 'slot' | 'poker' | 'baccarat' | 'dragon';
        game_id: string;
        bet_amount: number;
        win_amount: number;
        net: number;
        status: string;
        is_free_spin?: boolean;
        duration?: number;
        player_ids?: number[];
        winner_id?: number;
        hand_rank?: string;
        rake?: number;
        reel_result?: number[][];
        paylines_hit?: Array<{ line: number; symbols: number[]; payout: number }>;
        created_at: string;
      }>;
      total: number;
      page: number;
      page_size: number;
      has_more: boolean;
    }>("/game/manage/history", { params }),
};


// Lobby
export const lobbyApi = {
  getBanners: () => api.get<Banner[]>("/lobby/banners"),
  getCategories: () => api.get<Category[]>("/lobby/categories"),
  getGames: (params?: { category_id?: number; vendor_id?: number; keyword?: string; page?: number; page_size?: number }) =>
    api.get<GameListResponse>("/lobby/games", { params }),
  getConfig: () => api.get<Record<string, unknown>>("/lobby/config"),
  getSplash: () => api.get<Record<string, unknown>>("/lobby/splash"),
};

// Game
export const gameApi = {
  launch: (id: number) => api.get<{ game_url: string; launch_url: string; session_token: string; vendor: string; game_type?: string; game_id?: string; game_info_id?: number }>(`/game/launch/${id}`),
  getVendors: () => api.get<GameVendor[]>("/game/vendors"),
  toggleFavorite: (gameId: number) => api.post<{ is_favorite: boolean }>("/game/manage/favorite", { game_id: gameId }),
  getRecentGames: () => api.get<Game[]>("/game/manage/recent"),
  searchGames: (keyword: string, page?: number, pageSize?: number) =>
    api.get<{ list: Game[]; total: number }>("/game/manage/search", { params: { keyword, page, page_size: pageSize } }),
  endSession: (sessionId: string) => api.post<{ result: string }>("/game/manage/end-session", { session_id: sessionId }),
};

// Account
export const accountApi = {
  getProfile: () => api.get<UserProfile>("/account/profile"),
  updateProfile: (data: Partial<UserProfile>) => api.put<UserProfile>("/account/profile", data),
  getAssets: () => api.get<UserAssets>("/account/assets"),
  changePassword: (data: { old_password: string; new_password: string }) =>
    api.post<{ result: string }>("/account/change-password", data),
  uploadAvatar: (file: File) => {
    const formData = new FormData();
    formData.append("file", file);
    return api.post<{ avatar_url: string }>("/account/avatar", formData, {
      headers: { "Content-Type": "multipart/form-data" },
    });
  },
  deleteAccount: () => api.post<{ result: string }>("/account/delete-account"),
};

// VIP — original declaration was here; P1 replaced it with the typed
// version above (line ~295). Section marker kept so git diff is readable.

// Activity
export const activityApi = {
  getList: () => api.get<Activity[]>("/activity/list"),

  // Check-in
  checkIn: () => api.post<{ bonus_amount: number; consecutive_days: number }>("/activity/check-in"),
  getCheckInState: () => api.get<{ checked_today: boolean; consecutive_days: number; history: Array<{ date: string; bonus: number }> }>("/activity/check-in/state"),
  getCheckInConfig: () => api.get<{ daily_bonus: number; streak_bonuses: Record<number, number> }>("/activity/check-in/config"),

  // Recharge bonus
  claimRechargeBonus: () => api.post<{ bonus_amount: number }>("/activity/recharge-bonus"),

  // Timed gift
  claimTimedGift: () => api.post<{ item_id: number; item_name: string; quantity: number }>("/activity/timed-gift"),
  getTimedGiftStatus: () => api.get<{ available: boolean; next_available_at: string; cooldown_hours: number }>("/activity/timed-gift/status"),
};

// Shop
export const shopApi = {
  // Wallet balance
  getWallet: () => api.get<{
    balance: number;
    bonus_balance: number;
    frozen_balance: number;
    total_recharge: number;
    total_withdraw: number;
    recharge_count: number;
    withdraw_count: number;
    flow_required: number;
    flow_completed: number;
    currency: string;
  }>("/shop/wallet"),

  // Payment channels (for deposit)
  getPaymentChannels: () => api.get<unknown[]>("/shop/payment-channels"),

  // Withdraw channels
  getWithdrawChannels: () => api.get<unknown[]>("/shop/withdraw-channels"),

  // Create recharge (deposit) order
  recharge: (data: { channel_id: number; amount: number }) =>
    api.post<{ order_no: string; amount: number; status: string; pay_url?: string; pay_token?: string; qr_code?: string }>("/shop/recharge", data),

  // Create withdraw order
  withdraw: (data: { channel_id: number; amount: number; account?: string; account_name?: string }) =>
    api.post<{ order_no: string; amount: number; fee: number; real_amount: number; status: string }>("/shop/withdraw", data),

  // Order history (type: "recharge" | "withdraw" | "all")
  getOrders: (params?: { type?: string; page?: number; page_size?: number }) =>
    api.get<unknown[]>("/shop/orders", { params }),

  // User payment accounts (for withdrawal)
  getPaymentAccounts: () => api.get<unknown[]>("/shop/payment-accounts"),
  setPaymentAccount: (data: { id?: number; account_type: number; title: string; account: string; code?: string; username?: string }) =>
    api.post<{ id: number }>("/shop/payment-accounts", data),

  // Withdraw password
  setWithdrawPassword: (data: { old_pwd?: string; new_pwd: string }) =>
    api.post<{ result: string }>("/shop/withdraw-password", data),
};

export { TOKEN_KEY, REFRESH_TOKEN_KEY };

export interface ItemDefine {
  id: number;
  name: string;
  icon: string;
  type: number; // 1=consumable 2=time-limited 3=permanent
  duration: number;
  stackable: number;
  description: string;
  i18n_key: string;
}

export interface InventoryItem {
  id: number;
  item_id: number;
  name: string;
  icon: string;
  type: number;
  duration: number;
  stackable: number;
  quantity: number;
  description: string;
  expire_at: string | null;
  created_at: string;
}

// === Wheel / Lucky Spin ===
export interface WheelPrize {
  id: number;
  name: string;
  type: string; // "bonus" / "coin" / "item" / "empty"
  value: number;
  item_id: number;
  weight: number;
  icon: string;
  rarity: string; // "common" / "rare" / "epic" / "legendary"
  stock: number;
  remaining: number;
}

export interface WheelConfig {
  has_activity: boolean;
  activity_id: number;
  name: string;
  start_time: string;
  end_time: string;
  free_spins_per_day: number;
  spin_cost: number;
  spin_cost_type: string;
  cooldown_sec: number;
  max_spins_per_day: number;
  prizes: WheelPrize[];
}

export interface WheelState {
  has_activity: boolean;
  activity_id: number;
  remaining_free: number;
  total_spins: number;
  today_total_spins: number;
  cooldown_remaining: number;
  can_afford_paid: boolean;
  spin_cost: number;
  spin_cost_type: string;
  daily_limit_reached: boolean;
  history: Array<{
    prize_name: string;
    prize_type: string;
    prize_rarity: string;
    value: number;
    created_at: string;
  }>;
}

export interface SpinResult {
  spin_type: string;
  prize_index: number;
  prize: {
    id: number;
    name: string;
    type: string;
    value: number;
    item_id: number;
    rarity: string;
    icon: string;
  };
  total_spins: number;
  remaining_free: number;
  today_total_spins: number;
}

export const itemApi = {
  getInventory: () => api.get<InventoryItem[]>("/item/inventory"),
  getList: () => api.get<ItemDefine[]>("/item/list"),
  useItem: (data: { item_id: number; quantity?: number }) =>
    api.post<{ quantity: number }>("/item/use", data),
  transfer: (data: { target_user_id: number; item_id: number; quantity: number }) =>
    api.post<{ result: string }>("/item/transfer", data),
};

export const wheelApi = {
  getConfig: () => api.get<WheelConfig>("/activity/spin-wheel/config"),
  getState: () => api.get<WheelState>("/activity/spin-wheel/state"),
  spin: (useFree?: boolean) =>
    api.post<SpinResult>("/activity/spin-wheel", useFree !== undefined ? { use_free: useFree } : {}),
};

// ─── Task ────────────────────────────────────────────────────────────────────
export interface TaskItem {
  task_id: number;
  task_type: number;
  task_name: string;
  task_description: string;
  task_reward: number;
  task_status: number;
  task_progress: number;
  task_target: number;
  receive_status: number;
  link_url?: string;
  link_type?: string;
  task_icon?: string;
}

export interface TaskTypeState {
  task_type: number;
  receive_all_btn: number;
  task_type_state: TaskItem[];
}

export interface TaskProgress {
  task_list: TaskItem[];
  task_type: number;
  task_name: string;
}

export const taskApi = {
  getTaskConfig: async () => {
    const [daily, weekly, growth] = await Promise.allSettled([
      api.get<TaskItem[]>("/task/daily"),
      api.get<TaskItem[]>("/task/weekly"),
      api.get<TaskItem[]>("/task/growth"),
    ]);
    const getList = (r: PromiseSettledResult<{ data: TaskItem[] }>): TaskItem[] =>
      r.status === 'fulfilled' ? (r.value.data || []) : [];
    const wrap = (list: TaskItem[], type: number): TaskTypeState => ({
      task_type: type,
      receive_all_btn: list.some(t => t.receive_status === 1) ? 1 : 0,
      task_type_state: list,
    });
    return { data: [wrap(getList(daily), 0), wrap(getList(weekly), 1), wrap(getList(growth), 2)] };
  },
  getTaskProgress: () => api.get<TaskProgress[]>("/task/progress"),
  claimReward: (taskId: number) =>
    api.post<{ item_id: number; item_name: string; quantity: number }>("/task/claim", { task_id: taskId }),
  claimAllRewards: (taskType?: number) =>
    api.post<{ count: number; items: Array<{ item_id: number; item_name: string; quantity: number }> }>("/task/claim", taskType !== undefined ? { task_type: taskType } : {}),
};

// ─── Mail ────────────────────────────────────────────────────────────────────
export interface MailItem {
  mail_id: number;
  title: string;
  content: string;
  read_flag: number;
  receive_flag: number;
  is_collect: number;
  mail_type: number;
  from_name: string;
  created_at: string;
  expire_at: string;
  attachment?: Array<{
    item_id: number;
    item_name: string;
    quantity: number;
    icon: string;
  }>;
}

export interface MailListResult {
  mail_type: number;
  mail_count: number;
  new_mail_count: number;
  unread_mail_count: number;
  list: MailItem[];
}

export const mailApi = {
  getMailbox: () => api.get<MailListResult>("/mail/inbox"),
  readMail: (id: number) =>
    api.post<{ mail_id: number; title: string; content: string; attachment?: Array<{ item_id: number; item_name: string; quantity: number; icon: string }> }>("/mail/read", { mail_id: id }),
  deleteMail: (ids: number[]) =>
    api.post<{ result: string }>("/mail/delete", { mail_ids: ids }),
  claimMailAttachment: (id: number) =>
    api.post<{ items: Array<{ item_id: number; item_name: string; quantity: number }> }>("/mail/claim-attachment", { mail_id: id }),
  getUnreadCount: () =>
    api.get<{ unread_count: number }>("/mail/unread-count"),
};

// ─── Rank ────────────────────────────────────────────────────────────────────
export interface RankItem {
  rank: number;
  user_id: number;
  nickname: string;
  avatar: string;
  vip_level: number;
  total_amount: number;
}

export interface RankListResult {
  my_rank?: RankItem;
  rank_type: string;
  period: string;
  total_count: number;
  rank_list: RankItem[];
}

export const rankApi = {
  getRankList: (rankType: string, period?: string, page?: number) =>
    api.get<RankListResult>("/rank/list", { params: { rank_type: rankType, period, page } }),
  getMyRank: (rankType: string) =>
    api.get<{ my_rank: RankItem }>("/rank/my-rank", { params: { rank_type: rankType } }),
  getTopPlayers: (rankType: string, limit?: number) =>
    api.get<RankItem[]>("/rank/top", { params: { rank_type: rankType, limit } }),
};

// ─── Agent ───────────────────────────────────────────────────────────────────
export interface AgentInfo {
  user_id: number;
  nickname: string;
  agent_level: number;
  commission_rate: number;
  total_commission: number;
  available_commission: number;
  withdrawn_commission: number;
  subordinate_count: number;
  direct_subordinate_count: number;
  agent_status: number;
  referral_code: string;
  referral_link: string;
  created_at: string;
}

export interface SubordinateItem {
  user_id: number;
  nickname: string;
  avatar: string;
  vip_level: number;
  total_bet: number;
  commission: number;
  register_time: string;
  last_active_time: string;
}

export interface CommissionRecord {
  record_id: number;
  commission_type: number;
  amount: number;
  nickname: string;
  created_at: string;
}

export interface CommissionSummary {
  today_commission: number;
  yesterday_commission: number;
  this_month_commission: number;
  last_month_commission: number;
  total_commission: number;
  available_commission: number;
  pending_commission: number;
}

export const agentApi = {
  getAgentInfo: () => api.get<AgentInfo>("/agent/info"),
  getSubordinates: (page?: number, pageSize?: number) =>
    api.get<{ total: number; list: SubordinateItem[] }>("/agent/subordinates", { params: { page, page_size: pageSize } }),
  getCommissionRecords: (page?: number, pageSize?: number) =>
    api.get<{ total: number; list: CommissionRecord[] }>("/agent/commissions", { params: { page, page_size: pageSize } }),
  getDashboard: () => api.get<CommissionSummary>("/agent/dashboard"),
  requestSettlement: () => api.post<{ result: string }>("/agent/settlement"),
  getPromoLink: () => api.post<{ referral_code: string; referral_link: string }>("/agent/promo-link"),
};

// ─── Reddot ──────────────────────────────────────────────────────────────────
export interface ReddotState {
  categories: Record<string, number>;
  total: number;
}

export const reddotApi = {
  getReddots: () => api.get<ReddotState>("/lobby/reddot/state"),
  markAsRead: (category: string) =>
    api.post<{ status: string }>("/lobby/reddot/read", { category }),
};
