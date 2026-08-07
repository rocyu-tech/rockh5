'use client';

import { useEffect } from 'react';
import { toast } from 'sonner';
import { getWSRpcTransport } from '@/lib/ws-rpc-transport';
import { useAuthStore } from '@/store/auth';

/**
 * Listens for server→client push events on the WS RPC transport and reacts:
 *   - `balance_changed` → refresh assets so the UI reflects new balances.
 *   - `session.ready`  → session token is managed by the caller (game page);
 *                          nothing to do here yet.
 *   - unknown push    → ignored (extend the map as new events ship).
 *
 * Mounted once in the root layout inside AppProvider.
 */
export default function PushEvents() {
  useEffect(() => {
    return getWSRpcTransport().onPush((type, data) => {
      switch (type) {
        case 'balance_changed':
          useAuthStore.getState().fetchAssets().catch(() => {});
          break;
        case 'mail.updated':
          useAuthStore.getState().fetchUnreadMailCount().catch(() => {});
          break;
        case 'mail.new':
          useAuthStore.getState().fetchUnreadMailCount().catch(() => {});
          toast('You have a new mail');
          break;
        case 'session.ready':
          // Handled by the game bootstrap; no action needed here.
          break;
        default:
          // Unknown push — ignore so unknown events don't spam the UI.
          void data;
          break;
      }
    });
  }, []);

  return null;
}