import axios, { AxiosError, InternalAxiosRequestConfig } from "axios";

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
function forceLogout() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
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
  currency: string;
}

export interface VIPLevel {
  level: number;
  name: string;
  min_deposit: number;
  benefits: string[];
  icon?: string;
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
  launch: (id: number) => api.get<ApiResponse<{ game_url: string }>>(`/game/launch/${id}`),
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
  getPaymentChannels: () => api.get<ApiResponse<unknown[]>>("/shop/payment-channels"),
  recharge: (data: { channel_id: number; amount: number }) =>
    api.post<ApiResponse<{ order_no: string; pay_url: string }>>("/shop/recharge", data),
  withdraw: (data: { channel_id: number; amount: number; account?: string }) =>
    api.post<ApiResponse<{ order_no: string }>>("/shop/withdraw", data),
  getWithdrawChannels: () => api.get<ApiResponse<unknown[]>>("/shop/withdraw-channels"),
  getOrders: (params?: { page?: number; page_size?: number }) =>
    api.get<ApiResponse<unknown[]>>("/shop/orders", { params }),
};

export { TOKEN_KEY, REFRESH_TOKEN_KEY };

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

export const wheelApi = {
  getConfig: () => api.get<ApiResponse<WheelConfig>>("/activity/spin-wheel/config"),
  getState: () => api.get<ApiResponse<WheelState>>("/activity/spin-wheel/state"),
  spin: (useFree?: boolean) =>
    api.post<ApiResponse<SpinResult>>("/activity/spin-wheel", useFree !== undefined ? { use_free: useFree } : {}),
};
