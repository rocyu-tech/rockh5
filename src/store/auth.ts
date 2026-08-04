import { create } from "zustand";
import { authApi, accountApi, mailApi } from "@/lib/api";
import type { UserProfile, UserAssets } from "@/lib/api";
import { getErrorMessage } from "@/lib/api-status";

// Auth uses httpOnly cookies for actual auth (set by backend).
// A non-httpOnly mirror cookie (access_token) is also set client-side so that
// Next.js middleware can gate routes server-side. The mirror cookie has the
// same JWT value and matching MaxAge.
//
// flow:
//   1. Backend /auth/login sets httpOnly access_token + refresh_token cookies
//   2. Response body returns the JWT directly in `access_token` (no wrapper)
//   3. syncTokenCookie() writes a non-httpOnly mirror cookie for the middleware
//   4. On page refresh, middleware reads the mirror cookie → allows access
//   5. forceLogout() in api.ts clears the mirror cookie

/** Set or clear the non-httpOnly mirror cookie for Next.js middleware. */
function syncTokenCookie(token: string | null) {
  if (typeof document === "undefined") return;
  if (token) {
    // Mirror the backend's MaxAge (AccessTTL in minutes → seconds).
    // Default 24h if we don't know the exact TTL.
    document.cookie = `access_token=${token}; path=/; max-age=86400; samesite=lax`;
  } else {
    document.cookie = "access_token=; path=/; max-age=0";
  }
}

interface AuthState {
  user: UserProfile | null;
  assets: UserAssets | null;
  isLoggedIn: boolean;
  isLoading: boolean;
  lastError: string | null;
  unreadMailCount: number;
  login: (phone: string, password: string) => Promise<boolean>;
  register: (data: { phone: string; nickname: string; password: string; confirm_password: string }) => Promise<boolean>;
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
    //
    // Also ensure the middleware mirror cookie exists. If the user has a
    // valid httpOnly cookie but the mirror was lost (e.g. cookie overflow),
    // fetchProfile will succeed and we can re-sync from the response.
    if (typeof window !== "undefined") {
      get().fetchProfile();
      get().fetchAssets();
    }
  },

  login: async (phone: string, password: string) => {
    set({ isLoading: true });
    try {
      const res = await authApi.login(phone, password);
      // Backend sets httpOnly cookies. Also sync mirror cookie for middleware.
      syncTokenCookie(res.data.access_token || null);
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

  register: async (data: { phone: string; nickname: string; password: string; confirm_password: string }) => {
    set({ isLoading: true, lastError: null });
    try {
      const res = await authApi.register(data);
      // Backend sets httpOnly cookies. Also sync mirror cookie for middleware.
      syncTokenCookie(res.data.access_token || null);
      if (res.data.user_id) {
        set({
          isLoggedIn: true,
          isLoading: false,
        });
        get().fetchProfile();
        get().fetchAssets();
      } else {
        set({ isLoading: false });
      }
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
    syncTokenCookie(null);
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
