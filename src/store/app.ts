import { create } from "zustand";

// ── App-level UI signal store ──────────────────────────────────────────────
// Replaces CustomEvent-based communication with type-safe Zustand actions.
//
// Previously: components dispatched window events (auth:logout, nav:open-spin,
// nav:open-register) and AppProvider listened for them.
//
// Now: components call store actions directly, AppProvider subscribes to state.
//
// Benefits: type-safe, synchronous, no addEventListener cleanup,
// no stringly-typed event names, fully testable without DOM.

interface AppState {
  // Modal open requests — AppProvider subscribes and manages actual modal state
  openLoginRequested: boolean;
  openRegisterRequested: boolean;
  openSpinRequested: boolean;

  // Actions
  requestLogin: () => void;
  requestRegister: () => void;
  requestSpin: () => void;
  clearRequests: () => void;

  // Token refresh signal for WS clients
  // When the access token is rotated, the axios interceptor sets this.
  // ws.ts subscribes and calls refreshToken() on all active WS clients.
  lastRefreshedToken: string | null;
  setLastRefreshedToken: (token: string | null) => void;
}

export const useAppStore = create<AppState>((set) => ({
  openLoginRequested: false,
  openRegisterRequested: false,
  openSpinRequested: false,

  requestLogin: () => set({ openLoginRequested: true }),
  requestRegister: () => set({ openRegisterRequested: true }),
  requestSpin: () => set({ openSpinRequested: true }),
  clearRequests: () => set({ openLoginRequested: false, openRegisterRequested: false, openSpinRequested: false }),

  lastRefreshedToken: null,
  setLastRefreshedToken: (token) => set({ lastRefreshedToken: token }),
}));
