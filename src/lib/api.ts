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
// Fix: call useAuthStore.getState().logout() BEFORE dispatching the event.
// This synchronously clears isLoggedIn, so AppProvider's check passes and
// the login modal opens correctly.
function forceLogout() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
  // Synchronize zustand auth state (clears isLoggedIn, user, assets, etc.)
  useAuthStore.getState().logout();
  // Now dispatch the event — AppProvider will see isLoggedIn=false and
  // correctly open the login modal.
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

      const newToken = res.data?.data?.access_token;
      const newRefreshToken = res.data?.data?.refresh_token;

      if (newToken) {
        localStorage.setItem(TOKEN_KEY, newToken);
        if (newRefreshToken) {
          localStorage.setItem(REFRESH_TOKEN_KEY, newRefreshToken);
        }
        if (originalRequest.headers) {
          originalRequest.headers.Authorization = `Bearer ${newToken}`;
        }
        processQueue(null, newToken);
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
export interface ApiResponse<T = unknown> {
  code: number;
  message: string;
  data: T;
}

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
  category_id: number;
  category_name?: string;
  status: number;
  hot?: boolean;
  new?: boolean;
  tag?: string;
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
    api.post<ApiResponse<{ access_token: string; refresh_token: string }>>("/auth/login", { email, password }),

  register: (data: { email: string; password: string; confirm_password: string; phone?: string }) =>
    api.post<ApiResponse<{ access_token: string; refresh_token: string }>>("/auth/register", data),

  refresh: (refreshToken: string) =>
    api.post<ApiResponse<{ access_token: string; refresh_token: string }>>("/auth/refresh", { refresh_token: refreshToken }),
};

// Lobby
export const lobbyApi = {
  getBanners: () => api.get<ApiResponse<Banner[]>>("/lobby/banners"),
  getCategories: () => api.get<ApiResponse<Category[]>>("/lobby/categories"),
  getGames: (params?: { category_id?: number; vendor_id?: number; keyword?: string; page?: number; page_size?: number }) =>
    api.get<ApiResponse<GameListResponse>>("/lobby/games", { params }),
  getConfig: () => api.get<ApiResponse<Record<string, unknown>>>("/lobby/config"),
  getSplash: () => api.get<ApiResponse<Record<string, unknown>>>("/lobby/splash"),
};

// Game
export const gameApi = {
  launch: (id: number) => api.get<ApiResponse<{ game_url: string; launch_url: string; session_token: string; vendor: string }>>(`/game/launch/${id}`),
  getVendors: () => api.get<ApiResponse<GameVendor[]>>("/game/vendors"),
};

// Account
export const accountApi = {
  getProfile: () => api.get<ApiResponse<UserProfile>>("/account/profile"),
  updateProfile: (data: Partial<UserProfile>) => api.put<ApiResponse<UserProfile>>("/account/profile", data),
  getAssets: () => api.get<ApiResponse<UserAssets>>("/account/assets"),
};

// VIP
export const vipApi = {
  getLevels: () => api.get<ApiResponse<VIPLevel[]>>("/vip/levels"),
  getInfo: () => api.get<ApiResponse<{ level: number; progress: number }>>("/vip/info"),
};

// Activity
export const activityApi = {
  getList: () => api.get<ApiResponse<Activity[]>>("/activity/list"),
};

// Shop
export const shopApi = {
  // Wallet balance
  getWallet: () => api.get<ApiResponse<{
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
  }>>("/shop/wallet"),

  // Payment channels (for deposit)
  getPaymentChannels: () => api.get<ApiResponse<unknown[]>>("/shop/payment-channels"),

  // Withdraw channels
  getWithdrawChannels: () => api.get<ApiResponse<unknown[]>>("/shop/withdraw-channels"),

  // Create recharge (deposit) order
  recharge: (data: { channel_id: number; amount: number }) =>
    api.post<ApiResponse<{ order_no: string; amount: number; status: string; pay_url?: string; pay_token?: string; qr_code?: string }>>("/shop/recharge", data),

  // Create withdraw order
  withdraw: (data: { channel_id: number; amount: number; account?: string; account_name?: string }) =>
    api.post<ApiResponse<{ order_no: string; amount: number; fee: number; real_amount: number; status: string }>>("/shop/withdraw", data),

  // Order history (type: "recharge" | "withdraw" | "all")
  getOrders: (params?: { type?: string; page?: number; page_size?: number }) =>
    api.get<ApiResponse<unknown[]>>("/shop/orders", { params }),

  // User payment accounts (for withdrawal)
  getPaymentAccounts: () => api.get<ApiResponse<unknown[]>>("/shop/payment-accounts"),
  setPaymentAccount: (data: { id?: number; account_type: number; title: string; account: string; code?: string; username?: string }) =>
    api.post<ApiResponse<{ id: number }>>("/shop/payment-accounts", data),

  // Withdraw password
  setWithdrawPassword: (data: { old_pwd?: string; new_pwd: string }) =>
    api.post<ApiResponse<{ result: string }>>("/shop/withdraw-password", data),
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
  getInventory: () => api.get<ApiResponse<InventoryItem[]>>("/item/inventory"),
  getList: () => api.get<ApiResponse<ItemDefine[]>>("/item/list"),
  useItem: (data: { item_id: number; quantity?: number }) =>
    api.post<ApiResponse<{ quantity: number }>>("/item/use", data),
};

export const wheelApi = {
  getConfig: () => api.get<ApiResponse<WheelConfig>>("/activity/spin-wheel/config"),
  getState: () => api.get<ApiResponse<WheelState>>("/activity/spin-wheel/state"),
  spin: (useFree?: boolean) =>
    api.post<ApiResponse<SpinResult>>("/activity/spin-wheel", useFree !== undefined ? { use_free: useFree } : {}),
};
