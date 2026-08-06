import axios, { AxiosError, InternalAxiosRequestConfig } from "axios";
import { useAuthStore } from "@/store/auth";
import { useAppStore } from "@/store/app";
import { toast } from "sonner";

// Auth is handled entirely via httpOnly cookies (set by backend).
// No localStorage token storage. No Authorization header.
//
// WS still uses localStorage for ?token= query param (browser can't set cookies
// on WebSocket upgrade). See ws.ts for details.

// ── Shop types (gRPC-Gateway response shapes) ──────────────────────────────
export interface Channel {
  id: number;
  method_ids: string;
  name: string;
  type: string;
  min_amount: number;
  max_amount: number;
  fee_rate: number;
}

export interface WithdrawChannel extends Channel {
  daily_limit?: number;
}

// Payment method (returned by GET /shop/payment-methods)
export interface PaymentMethod {
  id: number;
  name: string;
  icon: string;
  scene: string; // "deposit" | "withdraw" | "both"
  min_amount: number;
  max_amount: number;
  sort_order: number;
  bonus_type?: number;
  bonus_value?: number;
  first_deposit_bonus_type?: number;
  first_deposit_bonus_value?: number;
}

// Shop product (deposit/withdraw amount option with bonus)
export interface ShopProduct {
  id: number;
  type: number;
  name: string;
  price: number;
  bonus_amount: number;
  currency: string;
  sort_order: number;
}

export interface PaymentAccount {
  id: number;
  bank_name: string;
  account_number: string;
  account_name: string;
  type: string;
}

export interface Order {
  id: number;
  order_no: string;
  type: string;
  amount: number;
  status: number;
  created_at: string;
}

export const api = axios.create({
  baseURL: "/api/v1",
  timeout: 15000,
  withCredentials: true, // P0-6: send httpOnly cookies (set by /auth/login) on every request
  headers: {
    "Content-Type": "application/json",
  },
});

// ── camelCase → snake_case response normalizer ──────────────────────────
// gRPC-Gateway proto3 default JSON uses lowerCamelCase (imageUrl, sortOrder…)
// but the frontend expects snake_case (image_url, sort_order…).
// This interceptor recursively converts ALL response keys so both
// the current (camelCase) and future (UseProtoNames) backend work.
function camelToSnake(str: string): string {
  return str.replace(/[A-Z]/g, (m) => `_${m.toLowerCase()}`);
}
function normalizeKeys(obj: unknown): unknown {
  if (obj === null || typeof obj !== 'object') return obj;
  if (Array.isArray(obj)) return obj.map(normalizeKeys);
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
    out[camelToSnake(k)] = normalizeKeys(v);
  }
  return out;
}

// ── Unified response unwrapper ──────────────────────────────────────────
// All backend responses now follow {code, message, data}.
// This interceptor extracts `data` on success (code===0) and rejects
// on error (code!==0), so callers receive the actual payload directly.
api.interceptors.response.use((response) => {
  const body = response.data;
  if (body && typeof body === 'object' && 'code' in body && 'data' in body) {
    if (body.code === 0 || body.code === '0') {
      // Success — unwrap: {code, message, data} → data
      response.data = body.data ?? {};
    } else {
      // Business error — reject with code + message
      const msg = body.message || 'request failed';
      if (typeof window !== 'undefined') {
        toast.error(msg);
      }
      const err = Object.assign(new Error(msg), {
        response: response,
        code: body.code,
      });
      return Promise.reject(err);
    }
  }
  // Apply camelCase → snake_case normalization to the unwrapped data
  if (response.data && typeof response.data === 'object') {
    response.data = normalizeKeys(response.data);
  }
  return response;
});

// Request interceptor — no Authorization header needed.
// Auth is sent via httpOnly cookie (withCredentials: true on the axios instance).
// This interceptor is kept as a no-op hook for future use if needed.
api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => config,
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

// Helper: clear auth state and request login dialog.
//
// Token is httpOnly cookie — we cannot clear it from JS.
// We clear the middleware mirror cookie so Next.js route guards release immediately,
// clear zustand auth state, and signal AppProvider to open the login modal.
function forceLogout() {
  // Clear the middleware cookie so Next.js route guards release immediately
  if (typeof document !== 'undefined') {
    document.cookie = 'access_token=; path=/; max-age=0';
    // Clear WS token from localStorage
    localStorage.removeItem('rockgame_token');
  }
  // Synchronously clear zustand auth state (no API call, no await)
  useAuthStore.setState({
    user: null,
    assets: null,
    isLoggedIn: false,
    lastError: null,
  });
  // Signal AppProvider to open login modal via Zustand (replaces CustomEvent)
  useAppStore.getState().requestLogin();
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
      // Show error toast for non-auth errors
      if (typeof window !== 'undefined' && !isAuthError) {
        const msg = (error.response?.data as { message?: string })?.message
          || error.message
          || 'Network error';
        toast.error(msg);
      }
      return Promise.reject(error);
    }

    // httpOnly cookie handles auth — we can't check if refresh token exists
    // from JS. Just try to refresh; if the server's refresh_token cookie is
    // missing/expired, the refresh call will fail and we forceLogout.

    if (isRefreshing) {
      return new Promise((resolve, reject) => {
        failedQueue.push({ resolve, reject });
      }).then(() => {
        return api(originalRequest);
      });
    }

    originalRequest._retry = true;
    isRefreshing = true;

    try {
      const res = await axios.post("/api/v1/auth/refresh", null, {
        withCredentials: true, // send httpOnly refresh cookie
      });

      const newToken = res.data?.access_token;

      if (newToken) {
        // Token rotation is handled entirely by httpOnly cookies.
        // Backend sets new access_token cookie on refresh response.
        // No localStorage writes needed.
        processQueue(null, newToken);
        // Notify WS clients of token refresh via Zustand (replaces CustomEvent)
        if (typeof window !== 'undefined') {
          useAppStore.getState().setLastRefreshedToken(newToken);
        }
        return api(originalRequest);
      }

      // Refresh returned no token — force logout
      const now = Date.now();
      if (now - lastLogoutTime > LOGOUT_COOLDOWN) {
        lastLogoutTime = now;
        forceLogout();
      }
      processQueue(error, null);
      return Promise.reject(error);
    } catch (refreshError) {
      // Refresh failed — force logout
      const now = Date.now();
      if (now - lastLogoutTime > LOGOUT_COOLDOWN) {
        lastLogoutTime = now;
        forceLogout();
      }
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
  login: (phone: string, password: string) =>
    api.post<{ user_id: number; phone: string; nickname: string; avatar: string; access_token: string; token_type: string; expires_in: number }>("/auth/login", { phone, password }),

  register: (data: { phone: string; nickname: string; password: string; confirm_password: string; invite_code?: string }) =>
    api.post<{ user_id: number; phone: string; nickname: string; access_token: string; token_type: string; expires_in: number }>("/auth/register", data),

  logout: () => api.post<{ result: string }>("/auth/logout"),

  // P1: password reset — 2-step flow.
  //   1. requestPasswordReset(email) → backend generates a 64-char hex token,
  //      stores it in Redis with 15min TTL, and (in dev) logs the token.
  //      In prod with SMTP wired, it emails a reset link.
  //   2. confirmPasswordReset(token, new_password) → validates token, sets
  //      new password, deletes token, force-logs-out all existing sessions.
  requestPasswordReset: (phone: string) =>
    api.post<{ message: string }>("/auth/password-reset/request", { phone }, { _noAuth: true } as object),
  confirmPasswordReset: (token: string, newPassword: string) =>
    api.post<{ message: string }>("/auth/password-reset/confirm", { token, new_password: newPassword }, { _noAuth: true } as object),
};

// Account — only uploadAvatar remains (other methods migrated to Connect RPC)
export const accountApi = {
  uploadAvatar: (file: File) => {
    const formData = new FormData();
    formData.append("file", file);
    return api.post<{ avatar_url: string }>("/account/avatar", formData, {
      headers: { "Content-Type": "multipart/form-data" },
    });
  },
};

// ── Inventory types ─────────────────────────────────────────────────────────
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

// ── Wheel / Lucky Spin types ────────────────────────────────────────────────
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

// ── Task types ──────────────────────────────────────────────────────────────
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

// ── Mail types ──────────────────────────────────────────────────────────────
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

// ── Rank types ──────────────────────────────────────────────────────────────
export interface RankItem {
  rank: number;
  user_id: number;
  nickname: string;
  avatar: string;
  vip_level: number;
  total_amount: number;
}

// ── Agent types ─────────────────────────────────────────────────────────────
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
