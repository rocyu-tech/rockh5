import { useState, useCallback, useRef } from 'react';
import { createContext, useContext } from 'react';
import { toast } from 'sonner';

export type ApiStatus = 'unknown' | 'online' | 'offline' | 'partial';

// Re-export context and hook so all components can import from one place
export const ApiStatusContext = createContext<ReturnType<typeof useApiStatus> | null>(null);

export function useApiStatusContext() {
  const ctx = useContext(ApiStatusContext);
  if (!ctx) throw new Error('useApiStatusContext must be used within ApiStatusContext.Provider');
  return ctx;
}

/**
 * Track backend API connectivity and show user-friendly toast notifications.
 *
 * - First failure: toast + set status
 * - Recovery: toast + set status
 * - Repeated failures within cooldown: silent
 */
export function useApiStatus() {
  const [status, setStatus] = useState<ApiStatus>('unknown');
  const [failedEndpoints, setFailedEndpoints] = useState<Set<string>>(new Set());
  const cooldownRef = useRef<Map<string, number>>(new Map());
  const COOLDOWN_MS = 30_000; // 30 seconds between repeated toasts for the same key

  const markSuccess = useCallback((endpoint: string) => {
    setFailedEndpoints((prev) => {
      const next = new Set(prev);
      next.delete(endpoint);
      if (next.size === 0) {
        setStatus((s) => (s === 'partial' || s === 'offline' ? 'online' : s));
      } else {
        setStatus('partial');
      }
      return next;
    });
    cooldownRef.current.delete(endpoint);
  }, []);

  const markFailed = useCallback(
    (endpoint: string, errorMessage?: string) => {
      const now = Date.now();
      const lastToast = cooldownRef.current.get(endpoint) ?? 0;

      setFailedEndpoints((prev) => {
        const next = new Set(prev);
        const isNew = !next.has(endpoint);
        next.add(endpoint);

        if (isNew && next.size === 1) {
          setStatus('offline');
        } else if (next.size > 0) {
          setStatus('partial');
        }
        return next;
      });

      // Show toast only if not in cooldown
      if (now - lastToast > COOLDOWN_MS) {
        cooldownRef.current.set(endpoint, now);

        const friendlyName = endpointToFriendly(endpoint);
        toast.error(`${friendlyName} unavailable`, {
          description: errorMessage || 'The server is not responding. Please try again later.',
          duration: 5000,
        });
      }
    },
    [],
  );

  const clearAll = useCallback(() => {
    setFailedEndpoints(new Set());
    setStatus('online');
    cooldownRef.current.clear();
  }, []);

  return {
    status,
    failedEndpoints,
    failedCount: failedEndpoints.size,
    isOffline: status === 'offline' || status === 'partial',
    markSuccess,
    markFailed,
    clearAll,
  };
}

function endpointToFriendly(endpoint: string): string {
  const map: Record<string, string> = {
    'lobby/banners': 'Banners',
    'lobby/categories': 'Game Categories',
    'lobby/games': 'Game List',
    'lobby/config': 'Lobby Config',
    'lobby/splash': 'Splash Popup',
    'game/launch': 'Game Launch',
    'game/vendors': 'Game Vendors',
    'account/profile': 'User Profile',
    'account/assets': 'Account Balance',
    'auth/login': 'Login',
    'auth/register': 'Registration',
    'auth/refresh': 'Token Refresh',
    'vip/info': 'VIP Info',
    'vip/levels': 'VIP Levels',
    'activity/list': 'Promotions',
    'activity/draw': 'Activity Draw',
    'shop/payment-channels': 'Payment Channels',
    'shop/recharge': 'Recharge',
    'shop/withdraw': 'Withdraw',
    'shop/orders': 'Order History',
    'task/list': 'Tasks',
    'task/claim': 'Task Claim',
    'mail/list': 'Mail',
    'mail/read': 'Mail Read',
    'mail/claim': 'Mail Claim',
    'rank/list': 'Rankings',
    'agent/info': 'Agent Info',
    'item/inventory': 'Inventory',
    'tag/list': 'Tags',
    'reddot/list': 'Notifications',
  };

  // Try to find the best match
  for (const [key, name] of Object.entries(map)) {
    if (endpoint.includes(key)) return name;
  }
  // Fallback: capitalize first segment
  const firstSegment = endpoint.split('/').filter(Boolean)[0] || 'API';
  return firstSegment.charAt(0).toUpperCase() + firstSegment.slice(1);
}

/**
 * Extract a user-friendly error message from an Axios error or unknown error.
 */
export function getErrorMessage(err: unknown): string {
  if (!err) return 'Unknown error';

  if (typeof err === 'string') return err;

  if (typeof err === 'object') {
    const e = err as Record<string, unknown>;

    // Axios error shape
    if ('response' in e && e.response) {
      const resp = e.response as Record<string, unknown>;
      const data = resp.data as Record<string, unknown> | undefined;
      if (data?.message && typeof data.message === 'string') return data.message;
      const status = resp.status as number | undefined;
      if (status === 401) return 'Session expired. Please login again.';
      if (status === 403) return 'Access denied.';
      if (status === 404) return 'Resource not found.';
      if (status === 429) return 'Too many requests. Please try again later.';
      if (status && status >= 500) return 'Server error. Please try again later.';
    }

    // Network error (no response)
    if ('code' in e && (e.code === 'ECONNABORTED' || e.code === 'ERR_NETWORK' || e.code === 'ETIMEDOUT')) {
      return 'Network timeout. Please check your connection.';
    }
    if ('message' in e && typeof e.message === 'string') {
      if (e.message.includes('Network Error')) return 'Network error. Server may be offline.';
      if (e.message.includes('timeout')) return 'Request timed out.';
      if (e.message.includes('aborted')) return 'Request cancelled.';
      return e.message;
    }
  }

  return 'Something went wrong. Please try again.';
}
