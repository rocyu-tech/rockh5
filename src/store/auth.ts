import { create } from "zustand";
import { authApi, accountApi, mailApi } from "@/lib/api";
import type { UserProfile, UserAssets } from "@/lib/api";
import { getErrorMessage } from "@/lib/api-status";

// Auth is now entirely httpOnly cookie based.
// No localStorage token storage. No client-side token in memory.
// The middleware cookie (access_token) is only needed for Next.js route guards
// and is set/cleared by api.ts forceLogout. We no longer need syncTokenCookie
// here because the backend /auth/login response sets the httpOnly cookie directly.

interface AuthState {
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
  user: null,
  assets: null,
  isLoggedIn: false,
  isLoading: false,
  lastError: null,
  unreadMailCount: 0,

  hydrate: () => {
    // With httpOnly cookies, we can't check if the user has a valid session
    // from client-side JS. Instead, we try to fetch the profile — if it
    // succeeds (cookie is valid), we're logged in. If it 401s, the axios
    // interceptor handles forceLogout and opens the login modal.
    if (typeof window !== "undefined") {
      get().fetchProfile();
      get().fetchAssets();
    }
  },

  login: async (email: string, password: string) => {
    set({ isLoading: true });
    try {
      const res = await authApi.login(email, password);
      // Backend sets httpOnly cookies on the login response.
      // No client-side token storage needed.
      set({
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
      // Backend sets httpOnly cookies on register response.
      // Token is NOT in the response body (httpOnly means XSS can't read it).
      if (res.data?.user_id) {
        set({
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
    // (clears httpOnly cookies server-side)
    try {
      const { api } = await import("@/lib/api");
      await api.post("/auth/logout");
    } catch (err) {
      // Best-effort: still clear local state even if logout API fails
      console.warn('[auth] logout API call failed:', err);
    }
    set({
      user: null,
      assets: null,
      isLoggedIn: false,
      lastError: null,
    });
  },

  fetchProfile: async () => {
    try {
      const res = await accountApi.getProfile();
      set({ user: res.data, isLoggedIn: true });
    } catch (err) {
      // Auth errors (401) are handled by the axios interceptor.
      // Log non-auth errors for debugging.
      const status = (err as { response?: { status?: number } })?.response?.status;
      if (status && status !== 401 && status !== 403) {
        console.warn('[auth] fetchProfile failed:', getErrorMessage(err));
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
        console.warn('[auth] fetchAssets failed:', getErrorMessage(err));
      }
    }
  },

  fetchUnreadMailCount: async () => {
    try {
      const res = await mailApi.getUnreadCount();
      set({ unreadMailCount: res.data?.unread_count || 0 });
    } catch (err) {
      console.warn('[auth] fetch unread mail count failed:', err);
    }
  },
}));
