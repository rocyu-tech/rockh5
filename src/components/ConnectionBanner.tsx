'use client';

import { Wifi, WifiOff, AlertTriangle, X } from 'lucide-react';
import { useState } from 'react';

interface ConnectionBannerProps {
  isOffline: boolean;
  failedCount: number;
  status: 'unknown' | 'online' | 'offline' | 'partial';
  onDismiss: () => void;
}

export default function ConnectionBanner({ isOffline, failedCount, status, onDismiss }: ConnectionBannerProps) {
  const [dismissed, setDismissed] = useState(false);

  if (!isOffline || dismissed) return null;

  const isFullOffline = status === 'offline';

  return (
    <div
      className={`fixed top-0 left-0 right-0 z-[60] transition-all duration-300 ${
        isFullOffline
          ? 'bg-gradient-to-r from-[#e94560]/90 to-[#e94560]/80'
          : 'bg-gradient-to-r from-[#f5a623]/90 to-[#f5a623]/80'
      } backdrop-blur-sm`}
    >
      <div className="max-w-7xl mx-auto px-4 py-2 flex items-center justify-between">
        <div className="flex items-center gap-2 text-sm">
          {isFullOffline ? (
            <WifiOff className="w-4 h-4 text-white flex-shrink-0" />
          ) : (
            <AlertTriangle className="w-4 h-4 text-[#0a0a1a] flex-shrink-0" />
          )}
          <span className={isFullOffline ? 'text-white font-medium' : 'text-[#0a0a1a] font-medium'}>
            {isFullOffline
              ? 'Server is currently unavailable. Showing demo data.'
              : `${failedCount} service${failedCount > 1 ? 's' : ''} unavailable. Some data may be demo content.`}
          </span>
        </div>
        <button
          onClick={() => {
            setDismissed(true);
            onDismiss();
          }}
          className="p-2 hover:bg-white/20 rounded-full transition-colors -mr-1"
          aria-label="Dismiss notification"
        >
          <X className="w-4 h-4 text-white/80" />
        </button>
      </div>
    </div>
  );
}
