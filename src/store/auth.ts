import { create } from "zustand";
import { authApi, accountApi, mailApi, TOKEN_KEY, REFRESH_TOKEN_KEY } from "@/lib/api";
import type { UserProfile, UserAssets } from "@/lib/api";
import { getErrorMessage } from "@/lib/api-status";

interface AuthState {
  token: string | null;
  refreshToken: string | null;
  user: UserProfile | null;
  assets: UserAssets | null;
  isLoggedIn: boolean;
  isLoading: boolean;
  lastError: string | null;
  unreadMailCount: number;
  login: (email: string, password: string) => Promise<boolean>;
  register: (data: { email: string; password: string; confirm_password: string; phone?: string }) => Promise<boolean>;
  logout: () => void;
  fetchProfile: () => Promise<void>;
  fetchAssets: () => Promise<void>;
  fetchUnreadMailCount: () => Promise<void>;
  hydrate: () => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: null,
  refreshToken: null,
  user: null,
  assets: null,
  isLoggedIn: false,
  isLoading: false,
  lastError: null,
  unreadMailCount: 0,

  hydrate: () => {
    if (typeof window !== "undefined") {
      const token = localStorage.getItem(TOKEN_KEY);
      const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY);
      if (token) {
        set({ token, refreshToken, isLoggedIn: true });
        get().fetchProfile();
        get().fetchAssets();
      }
    }
  },

  login: async (email: string, password: string) => {
    set({ isLoading: true });
    try {
      const res = await authApi.login(email, password);
      const data = res.data;
      localStorage.setItem(TOKEN_KEY, data.access_token);
      localStorage.setItem(REFRESH_TOKEN_KEY, data.refresh_token);
      set({
        token: data.access_token,
        refreshToken: data.refresh_token,
        isLoggedIn: true,
        isLoading: false,
      });
      get().fetchProfile();
      get().fetchAssets();
      return true;
    } catch (err) {
      set({ isLoading: false, lastError: getErrorMessage(err) });
      return false;
    }
  },

  register: async (data) => {
    set({ isLoading: true, lastError: null });
    try {
      const res = await authApi.register(data);
      const tokenData = res.data;
      if (tokenData?.access_token) {
        localStorage.setItem(TOKEN_KEY, tokenData.access_token);
        localStorage.setItem(REFRESH_TOKEN_KEY, tokenData.refresh_token);
        set({
          token: tokenData.access_token,
          refreshToken: tokenData.refresh_token,
          isLoggedIn: true,
          isLoading: false,
        });
        get().fetchProfile();
        get().fetchAssets();
      } else {
        set({ isLoading: false });
      }
      return true;
    } catch (err) {
      set({ isLoading: false, lastError: getErrorMessage(err) });
      return false;
    }
  },

  logout: async () => {
    // Call backend logout API to invalidate token server-side
    try {
      const { api } = await import("@/lib/api");
      await api.post("/auth/logout");
    } catch {
      // Ignore logout API errors — still clear local state
    }
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(REFRESH_TOKEN_KEY);
    set({
      token: null,
      refreshToken: null,
      user: null,
      assets: null,
      isLoggedIn: false,
      lastError: null,
    });
  },

  fetchProfile: async () => {
    try {
      const res = await accountApi.getProfile();
      set({ user: res.data });
    } catch (err) {
      // Auth errors (401) are handled by the axios interceptor.
      // Log non-auth errors for debugging.
      const status = (err as { response?: { status?: number } })?.response?.status;
      if (status && status !== 401 && status !== 403) {
        console.warn('[auth] fetchProfile failed:', status);
      }
    }
  },

  fetchAssets: async () => {
    try {
      const res = await accountApi.getAssets();
      set({ assets: res.data });
    } catch (err) {
      const status = (err as { response?: { status?: number } })?.response?.status;
      if (status && status !== 401 && status !== 403) {
        console.warn('[auth] fetchAssets failed:', status);
      }
    }
  },

  fetchUnreadMailCount: async () => {
    try {
      const res = await mailApi.getUnreadCount();
      set({ unreadMailCount: res.data?.unread_count || 0 });
    } catch {
      // Silently fail
    }
  },
}));
