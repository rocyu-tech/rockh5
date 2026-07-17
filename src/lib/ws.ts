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
  }

  connect() {
    this.isManualClose = false;
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
    console.log(`[ws] reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
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
  }

  get isConnected() {
    return this.ws?.readyState === WebSocket.OPEN;
  }
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
