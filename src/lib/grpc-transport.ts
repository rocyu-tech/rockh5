/**
 * gRPC-Web Transport Layer for H5 → Gate (Connect protocol)
 *
 * Architecture:
 *   H5 (Connect protocol, application/proto)
 *     → Gate (connectproxy: frame-level passthrough, no proto awareness)
 *       → mesh/node (standard gRPC)
 *
 * Auth flow:
 *   - Login/Register/Refresh/Logout: stay as REST (Fiber handlers in Gate)
 *   - All business RPCs: use this transport (httpOnly cookies sent automatically)
 *
 * The Connect proxy in Gate extracts user identity from the httpOnly cookie,
 * so we do NOT set Authorization header here.
 *
 * Connect protocol error handling:
 *   Gate returns HTTP 200 with X-Grpc-Status/X-Grpc-Message headers on error.
 *   The @connectrpc/connect client library automatically parses these into
 *   ConnectError objects, which our callers catch via handleRpc().
 */

import { createConnectTransport } from "@connectrpc/connect-web";
import { Code, ConnectError } from "@connectrpc/connect";
import type { Interceptor } from "@connectrpc/connect";
import { toast } from "sonner";
import { useAuthStore } from "@/store/auth";
import { useAppStore } from "@/store/app";

// ── Force logout helpers (shared with axios api.ts) ───────────────────

let lastLogoutTime = 0;
const LOGOUT_COOLDOWN = 5000; // 5 seconds cooldown between login popups

function forceLogout() {
  if (typeof document !== "undefined") {
    document.cookie = "access_token=; path=/; max-age=0";
  }
  useAuthStore.setState({
    user: null,
    assets: null,
    isLoggedIn: false,
    lastError: null,
  });
  useAppStore.getState().requestLogin();
}

// ── Token refresh (REST call to Gate's Fiber handler) ─────────────────

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

async function refreshAuthToken(): Promise<boolean> {
  try {
    const res = await fetch("/api/v1/auth/refresh", {
      method: "POST",
      credentials: "include", // send httpOnly refresh cookie
      headers: { "Content-Type": "application/json" },
    });
    if (!res.ok) return false;
    const body = await res.json();
    const newToken = body?.data?.access_token;
    if (newToken) {
      if (typeof window !== "undefined") {
        useAppStore.getState().setLastRefreshedToken(newToken);
      }
      return true;
    }
    return false;
  } catch {
    return false;
  }
}

// ── Connect Interceptor for auth refresh + error toast ─────────────────

/**
 * Interceptor that handles:
 * 1. 401 responses → refresh token → retry once
 * 2. Non-auth ConnectError → toast error message
 *
 * Note: Gate's connectproxy returns HTTP 200 with Connect error headers
 * for gRPC-level errors. The Connect client parses these BEFORE our
 * interceptor sees the response, so ConnectErrors are thrown as exceptions.
 * We catch those in the post-response side effect via the next() rejection.
 *
 * For HTTP-level 401 (not wrapped in Connect), the interceptor retries.
 */
function authInterceptor(): Interceptor {
  return (next) => async (req) => {
    // Skip retry if already retried
    if (req.header.get("x-grpc-retried")) {
      return next(req);
    }

    try {
      return await next(req);
    } catch (err) {
      if (!(err instanceof ConnectError)) {
        // Network error, timeout, etc.
        if (typeof window !== "undefined") {
          toast.error(err instanceof Error ? err.message : "Network error");
        }
        throw err;
      }

      // ConnectError: Gate returned gRPC error via Connect protocol
      if (err.code === Code.Unauthenticated) {
        // Try to refresh and retry
        if (isRefreshing) {
          return new Promise<never>((resolve, reject) => {
            failedQueue.push({ resolve: resolve as (value: unknown) => void, reject });
          });
        }

        isRefreshing = true;
        const refreshed = await refreshAuthToken();
        isRefreshing = false;

        if (refreshed) {
          processQueue(null);
          // Retry original request
          req.header.set("x-grpc-retried", "1");
          return next(req);
        }

        // Refresh failed — force logout
        processQueue(err);
        const now = Date.now();
        if (now - lastLogoutTime > LOGOUT_COOLDOWN) {
          lastLogoutTime = now;
          forceLogout();
        }
        throw err;
      }

      // Non-auth error: show toast
      if (typeof window !== "undefined") {
        toast.error(err.message || "Request failed");
      }
      throw err;
    }
  };
}

// ── Create the Connect transport ───────────────────────────────────────

/**
 * Connect transport for gRPC-Web calls through Gate.
 *
 * - baseUrl: "" → requests go to same origin (Gate)
 * - useBinaryFormat: true → sends application/proto (matches connectproxy's handleConnect)
 * - credentials via custom fetch wrapper (include cookies)
 *
 * Usage:
 *   import { createClient } from "@connectrpc/connect";
 *   import { ShopService } from "@/proto/shop_connect";
 *   const client = createClient(ShopService, grpcTransport);
 *   const res = await client.getShopWallet({});
 */
export const grpcTransport = createConnectTransport({
  baseUrl: "",
  useBinaryFormat: true,
  interceptors: [authInterceptor()],
  defaultTimeoutMs: 15_000,
  // Custom fetch with credentials: "include" for httpOnly cookies
  fetch: (input, init) =>
    globalThis.fetch(input, {
      ...init,
      credentials: "include",
    }),
});

/**
 * Create a typed service client bound to our transport.
 * Re-exported for convenience.
 *
 * Usage:
 *   import { grpcClient } from "@/lib/grpc-transport";
 *   import { ShopService } from "@/proto/shop_connect";
 *   const shopClient = grpcClient(ShopService);
 *   const wallet = await shopClient.getShopWallet({});
 */
export { createClient as grpcClient } from "@connectrpc/connect";

// Re-export ConnectError and Code for callers that need fine-grained handling
export { ConnectError, Code } from "@connectrpc/connect";
