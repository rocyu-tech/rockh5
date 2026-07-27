// WebSocket client manager for game connections.
//
// Features:
// - Auto-reconnect with exponential backoff (1s → 2s → 4s → max 10s)
// - Heartbeat ping every 30s (matches server expectation)
// - Message queue: messages sent before connection open are queued
// - Request-response matching via req_id
// - Push message subscription (for room_ready, round_end, etc.)

type WSMessageHandler = (action: string, data: unknown) => void;

interface WSOptions {
  url: string;
  token: string;
  onOpen?: () => void;
  onClose?: () => void;
  onError?: (err: Event) => void;
}

// P0-7: registry of active WS clients so the global 'auth:token-refreshed'
// listener can update all of them when the access token is rotated.
const activeClients = new Set<GameWSClient>();

export class GameWSClient {
  private ws: WebSocket | null = null;
  private url: string;
  private token: string;
  private queue: string[] = [];
  private reqIdCounter = 0;
  private pendingRequests = new Map<string, { resolve: (data: unknown) => void; reject: (err: Error) => void }>();
  private pushHandlers = new Map<string, Set<WSMessageHandler>>();
  private reconnectAttempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private isManualClose = false;
  private onOpenCb?: () => void;
  private onCloseCb?: () => void;
  private onErrorCb?: (err: Event) => void;

  constructor(opts: WSOptions) {
    this.url = opts.url;
    this.token = opts.token;
    this.onOpenCb = opts.onOpen;
    this.onCloseCb = opts.onClose;
    this.onErrorCb = opts.onError;
    // P0-7: register this client so it receives token refresh events.
    activeClients.add(this);
  }

  connect() {
    this.isManualClose = false;
    // P0-7: re-read token from localStorage on every connect so reconnects
    // don't reuse a stale token (the axios interceptor may have rotated it
    // since this client was constructed).
    if (typeof window !== 'undefined') {
      const fresh = localStorage.getItem('rockgame_token');
      if (fresh) this.token = fresh;
    }
    const wsUrl = `${this.url}?token=${encodeURIComponent(this.token)}`;
    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      // Flush queued messages
      while (this.queue.length > 0) {
        const msg = this.queue.shift()!;
        this.ws?.send(msg);
      }
      // Start heartbeat
      this.heartbeatTimer = setInterval(() => {
        this.sendRaw(JSON.stringify({ action: "ping", req_id: `hb_${Date.now()}` }));
      }, 30000);
      this.onOpenCb?.();
    };

    this.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        // Handle response to a request
        if (msg.req_id && this.pendingRequests.has(msg.req_id)) {
          const req = this.pendingRequests.get(msg.req_id)!;
          this.pendingRequests.delete(msg.req_id);
          if (msg.action === "error") {
            req.reject(new Error(msg.data?.message || "Unknown error"));
          } else {
            req.resolve(msg.data);
          }
          return;
        }
        // Handle push message
        if (msg.action) {
          const handlers = this.pushHandlers.get(msg.action);
          if (handlers) {
            handlers.forEach((h) => h(msg.action, msg.data));
          }
        }
      } catch (e) {
        console.error("[ws] parse message failed:", e);
      }
    };

    this.ws.onerror = (err) => {
      this.onErrorCb?.(err);
    };

    this.ws.onclose = () => {
      if (this.heartbeatTimer) {
        clearInterval(this.heartbeatTimer);
        this.heartbeatTimer = null;
      }
      this.onCloseCb?.();
      if (!this.isManualClose) {
        this.scheduleReconnect();
      }
    };
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 10000);
    this.reconnectAttempts++;
    this.reconnectTimer = setTimeout(() => this.connect(), delay);
  }

  // Send a message and wait for response (returns a Promise)
  request(action: string, data?: unknown): Promise<unknown> {
    return new Promise((resolve, reject) => {
      const reqId = `req_${++this.reqIdCounter}_${Date.now()}`;
      this.pendingRequests.set(reqId, { resolve, reject });
      const msg = JSON.stringify({ action, data, req_id: reqId });
      this.sendRaw(msg);
      // Timeout after 10s
      setTimeout(() => {
        if (this.pendingRequests.has(reqId)) {
          this.pendingRequests.delete(reqId);
          reject(new Error("request timeout"));
        }
      }, 10000);
    });
  }

  // Send a fire-and-forget message (no response expected)
  send(action: string, data?: unknown) {
    const msg = JSON.stringify({ action, data });
    this.sendRaw(msg);
  }

  private sendRaw(msg: string) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(msg);
    } else {
      this.queue.push(msg);
    }
  }

  // Subscribe to push messages
  on(action: string, handler: WSMessageHandler): () => void {
    if (!this.pushHandlers.has(action)) {
      this.pushHandlers.set(action, new Set());
    }
    this.pushHandlers.get(action)!.add(handler);
    return () => this.pushHandlers.get(action)?.delete(handler);
  }

  close() {
    this.isManualClose = true;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    if (this.heartbeatTimer) clearInterval(this.heartbeatTimer);
    this.ws?.close();
    this.ws = null;
    // P0-7: unregister so we no longer receive token refresh events.
    activeClients.delete(this);
  }

  get isConnected() {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  // P0-7: refresh the token used for WebSocket auth.
  //
  // Browsers cannot set the Authorization header on a native WebSocket()
  // upgrade request, so the JWT is passed as ?token= in the query string.
  // The token is captured at construction time and reused for reconnects —
  // but access tokens expire every 15 minutes. Without this method, a
  // long-lived poker session would lose its WS connection on the next
  // reconnect attempt (the stale token gets 401).
  //
  // The axios response interceptor calls this method (via the global
  // 'auth:token-refreshed' window event) whenever the access token is
  // rotated. We update this.token and, if the connection is currently
  // OPEN, do nothing (the existing connection is still valid until the
  // server closes it). If we're in a reconnect backoff, the next
  // connect() call will pick up the new token via the localStorage read
  // in connect().
  refreshToken(newToken: string) {
    if (!newToken || newToken === this.token) return;
    this.token = newToken;
    // No need to reconnect if currently OPEN — the existing connection
    // was authenticated with the old (still-valid) token. The new token
    // will be used on the next connect() cycle.
  }
}

// P0-7: subscribe to global 'auth:token-refreshed' events so all
// GameWSClient instances update their token when axios rotates it.
// The registry is declared at the top of this file (activeClients).
if (typeof window !== 'undefined') {
  window.addEventListener('auth:token-refreshed', (event: Event) => {
    const detail = (event as CustomEvent).detail as { token: string } | undefined;
    if (detail?.token) {
      activeClients.forEach((c) => c.refreshToken(detail.token));
    }
  });
}

// Helper: build WS URL from HTTP backend URL
export function getWSBaseURL(): string {
  const backend = process.env.NEXT_PUBLIC_BACKEND_URL || "http://localhost:8880";
  return backend.replace(/^http/, "ws");
}

// Helper: get auth token from localStorage
export function getAuthToken(): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem("rockgame_token") || "";
}
