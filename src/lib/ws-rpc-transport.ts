/**
 * WS RPC Transport — WebSocket-based generic RPC client.
 *
 * Protocol:
 *   Request:  { id, service, method, data: "<base64 pb>" }
 *   Response: { id, data: "<base64 pb>" }
 *   Error:    { id, error: { code, message } }
 *   Push:     { id: 0, push: "<type>", data: "<base64 pb>" }
 *
 * Auth: httpOnly cookies are sent automatically on WS upgrade (same-origin).
 * Token refresh: if gRPC UNAUTHENTICATED (code 16), close WS, refresh via REST,
 *   then reconnect on next call.
 */

import type { Message, MessageType } from "@bufbuild/protobuf";
import { toBinary, fromBinary } from "@bufbuild/protobuf";
import { toast } from "sonner";
import { useAuthStore } from "@/store/auth";
import { useAppStore } from "@/store/app";

// ── Types ───────────────────────────────────────────────────────────────

interface WSRequest {
  id: number;
  service: string;
  method: string;
  data: string; // base64-encoded protobuf
}

interface WSResponse {
  id: number;
  data?: string; // base64-encoded protobuf
  error?: { code: number; message: string };
  push?: string; // push type (when id=0)
}

interface PendingRequest {
  resolve: (data: Uint8Array) => void;
  reject: (error: WSRpcError) => void;
  timer: ReturnType<typeof setTimeout>;
}

export class WSRpcError extends Error {
  code: number;
  constructor(code: number, message: string) {
    super(message);
    this.name = "WSRpcError";
    this.code = code;
  }
}

// ── Push handler type ───────────────────────────────────────────────────

type PushHandler = (pushType: string, data: Uint8Array) => void;

// ── Singleton Transport ─────────────────────────────────────────────────

const REQUEST_TIMEOUT = 15_000; // 15s per-request timeout
const RECONNECT_BASE = 1000;    // 1s initial backoff
const RECONNECT_MAX = 16_000;   // 16s max backoff
const RECONNECT_JITTER = 0.2;   // ±20% jitter

let instance: WSRpcTransport | null = null;

export function getWSRpcTransport(): WSRpcTransport {
  if (!instance) {
    instance = new WSRpcTransport();
  }
  return instance;
}

/** Reset the singleton (e.g. after logout). */
export function resetWSRpcTransport() {
  if (instance) {
    instance.close();
    instance = null;
  }
}

// ── WSRpcTransport ──────────────────────────────────────────────────────

export class WSRpcTransport {
  private ws: WebSocket | null = null;
  private pending = new Map<number, PendingRequest>();
  private nextId = 1;
  private pushHandlers = new Set<PushHandler>();
  private closed = false;
  private connectPromise: Promise<void> | null = null;
  private reconnectAttempt = 0;
  private lastTokenHash = ""; // detect token change for reconnect

  // ── Public API ───────────────────────────────────────────────────────

  /**
   * Invoke a gRPC method via WebSocket.
   * Returns decoded protobuf response bytes — caller is responsible for fromBinary().
   */
  async invokeRaw(service: string, method: string, reqData: Uint8Array): Promise<Uint8Array> {
    await this.ensureConnected();
    return this.sendRequest(service, method, reqData);
  }

  /**
   * Type-safe invoke: encodes request, decodes response.
   */
  async invoke<T extends Message>(
    service: string,
    method: string,
    request: Message,
    RespType: MessageType<T>,
  ): Promise<T> {
    const reqData = toBinary(request);
    const respData = await this.invokeRaw(service, method, reqData);
    return fromBinary(RespType, respData);
  }

  /** Register a push handler. Returns unsubscribe function. */
  onPush(handler: PushHandler): () => void {
    this.pushHandlers.add(handler);
    return () => this.pushHandlers.delete(handler);
  }

  /** Force close the connection. */
  close(): void {
    this.closed = true;
    this.cleanupWs();
  }

  /** Check if connected. */
  get isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }

  // ── Connection Management ────────────────────────────────────────────

  private async ensureConnected(): Promise<void> {
    // If connected and healthy, return immediately
    if (this.isConnected) {
      return;
    }

    // If already connecting, wait for that
    if (this.connectPromise) {
      return this.connectPromise;
    }

    this.connectPromise = this.connect();
    try {
      await this.connectPromise;
    } finally {
      this.connectPromise = null;
    }
  }

  private connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      // Build WS URL — same origin, cookies sent automatically
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const url = `${protocol}//${window.location.host}/rpc`;

      const ws = new WebSocket(url);
      ws.binaryType = "arraybuffer";

      const timeout = setTimeout(() => {
        ws.close();
        reject(new Error("WS connect timeout"));
      }, 10_000);

      ws.onopen = () => {
        clearTimeout(timeout);
        this.ws = ws;
        this.reconnectAttempt = 0;
        this.onOpen();
        resolve();
      };

      ws.onclose = (event) => {
        clearTimeout(timeout);
        this.onClose(event.code, event.reason);
        // Only reject if we were waiting for this connection
        if (this.connectPromise) {
          reject(new Error(`WS closed: ${event.code} ${event.reason}`));
        }
      };

      ws.onerror = () => {
        clearTimeout(timeout);
        reject(new Error("WS connect error"));
      };

      ws.onmessage = (event) => {
        this.onMessage(event.data as string);
      };
    });
  }

  private onOpen(): void {
    // Connection established — good place for future heartbeat
  }

  private onClose(code: number, reason: string): void {
    const wasConnected = this.ws !== null;
    this.ws = null;

    // Reject all pending requests
    for (const [id, pending] of this.pending) {
      clearTimeout(pending.timer);
      pending.reject(new WSRpcError(14, "connection closed"));
      this.pending.delete(id);
    }

    // Attempt reconnect (unless intentional close)
    if (this.closed || code === 1000) {
      return;
    }

    // If UNAUTHENTICATED (code 16) was the cause, try token refresh first
    // The actual auth error is handled per-request; here we just clean up.
    if (wasConnected && !this.closed) {
      this.scheduleReconnect();
    }
  }

  private scheduleReconnect(): void {
    if (this.closed) return;

    const delay = Math.min(
      RECONNECT_BASE * Math.pow(2, this.reconnectAttempt) * (1 + (Math.random() - 0.5) * 2 * RECONNECT_JITTER),
      RECONNECT_MAX,
    );
    this.reconnectAttempt++;

    setTimeout(() => {
      if (this.closed) return;
      this.ensureConnected().catch(() => {
        // Reconnect failed — scheduleReconnect will be called by onClose
      });
    }, delay);
  }

  // ── Request / Response ───────────────────────────────────────────────

  private sendRequest(service: string, method: string, data: Uint8Array): Promise<Uint8Array> {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      return Promise.reject(new WSRpcError(14, "not connected"));
    }

    const id = this.nextId++;
    const base64 = uint8ArrayToBase64(data);

    const envelope: WSRequest = {
      id,
      service,
      method,
      data: base64,
    };

    return new Promise<Uint8Array>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new WSRpcError(4, "request timeout"));
      }, REQUEST_TIMEOUT);

      this.pending.set(id, { resolve, reject, timer });

      try {
        this.ws!.send(JSON.stringify(envelope));
      } catch (err) {
        clearTimeout(timer);
        this.pending.delete(id);
        reject(new WSRpcError(2, "send failed"));
      }
    });
  }

  private onMessage(raw: string): void {
    let msg: WSResponse;
    try {
      msg = JSON.parse(raw);
    } catch {
      return; // ignore malformed messages
    }

    // Push message (id=0)
    if (msg.id === 0 && msg.push) {
      if (msg.data) {
        const data = base64ToUint8Array(msg.data);
        for (const handler of this.pushHandlers) {
          try {
            handler(msg.push, data);
          } catch (err) {
            console.error("[WS-RPC] push handler error:", err);
          }
        }
      }
      return;
    }

    // Response to a pending request
    const pending = this.pending.get(msg.id);
    if (!pending) {
      return; // stale response, ignore
    }

    this.pending.delete(msg.id);
    clearTimeout(pending.timer);

    if (msg.error) {
      const err = new WSRpcError(msg.error.code, msg.error.message);

      // Handle UNAUTHENTICATED → try token refresh
      if (msg.error.code === 16) {
        this.handleAuthError(err);
      } else {
        // Show toast for non-auth errors
        if (typeof window !== "undefined") {
          toast.error(msg.error.message || "Request failed");
        }
      }

      pending.reject(err);
      return;
    }

    if (!msg.data) {
      pending.reject(new WSRpcError(2, "empty response data"));
      return;
    }

    pending.resolve(base64ToUint8Array(msg.data));
  }

  // ── Auth Error Handling ──────────────────────────────────────────────

  private async handleAuthError(err: WSRpcError): Promise<void> {
    // Close current connection
    this.cleanupWs();

    // Try to refresh token via REST
    try {
      const res = await fetch("/api/v1/auth/refresh", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      });
      if (res.ok) {
        const body = await res.json();
        if (body?.access_token && typeof window !== "undefined") {
          useAppStore.getState().setLastRefreshedToken(body.access_token);
        }
        return; // Success — will reconnect on next invoke
      }
    } catch {
      // Refresh failed
    }

    // Refresh failed — force logout
    const now = Date.now();
    if (now - (this as any)._lastLogoutTime > 5000) {
      (this as any)._lastLogoutTime = now;
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
  }

  // ── Cleanup ──────────────────────────────────────────────────────────

  private cleanupWs(): void {
    if (this.ws) {
      this.ws.onclose = null; // prevent reconnect
      this.ws.close(1000, "cleanup");
      this.ws = null;
    }
  }
}

// ── Base64 helpers (browser-native) ─────────────────────────────────────

function uint8ArrayToBase64(bytes: Uint8Array): string {
  // Use browser native API for performance
  if (typeof globalThis.Buffer !== "undefined") {
    return globalThis.Buffer.from(bytes).toString("base64");
  }
  // Fallback for browser without Buffer
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]!);
  }
  return btoa(binary);
}

function base64ToUint8Array(base64: string): Uint8Array {
  if (typeof globalThis.Buffer !== "undefined") {
    return new Uint8Array(globalThis.Buffer.from(base64, "base64"));
  }
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}
