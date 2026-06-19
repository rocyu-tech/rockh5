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

api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as InternalAxiosRequestConfig & {
      _retry?: boolean;
    };

    const status = error.response?.status;
    const data = error.response?.data as { code?: number };

    if (
      (status === 401 || data?.code === 10003 || data?.code === 20005 || data?.code === 20006) &&
      !originalRequest._retry
    ) {
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

      const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY);
      if (!refreshToken) {
        isRefreshing = false;
        processQueue(error, null);
        localStorage.removeItem(TOKEN_KEY);
        localStorage.removeItem(REFRESH_TOKEN_KEY);
        window.dispatchEvent(new CustomEvent("auth:logout"));
        return Promise.reject(error);
      }

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
      } catch (refreshError) {
        processQueue(refreshError, null);
        localStorage.removeItem(TOKEN_KEY);
        localStorage.removeItem(REFRESH_TOKEN_KEY);
        window.dispatchEvent(new CustomEvent("auth:logout"));
        return Promise.reject(refreshError);
      } finally {
        isRefreshing = false;
      }
    }

    return Promise.reject(error);
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
  getOrders: (params?: { page?: number; page_size?: number }) =>
    api.get<ApiResponse<unknown[]>>("/shop/orders", { params }),
};

export { TOKEN_KEY, REFRESH_TOKEN_KEY };
