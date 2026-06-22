'use client';

import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/store/auth';
import { useApiStatusContext } from '@/lib/api-status';
import Navbar from '@/components/Navbar';
import { itemApi, InventoryItem } from '@/lib/api';
import { Package, RefreshCw, Loader2, Zap, Clock, Shield } from 'lucide-react';

const TYPE_CONFIG: Record<number, { label: string; color: string; icon: React.ElementType; bg: string }> = {
  1: { label: 'Consumable', color: 'text-green-400', icon: Zap, bg: 'bg-green-500/10' },
  2: { label: 'Time-limited', color: 'text-blue-400', icon: Clock, bg: 'bg-blue-500/10' },
  3: { label: 'Permanent', color: 'text-purple-400', icon: Shield, bg: 'bg-purple-500/10' },
};

function formatExpiry(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

function isExpiringSoon(dateStr: string): boolean {
  const diff = new Date(dateStr).getTime() - Date.now();
  return diff > 0 && diff < 7 * 24 * 60 * 60 * 1000;
}

export default function InventoryPage() {
  const router = useRouter();
  const [items, setItems] = useState<InventoryItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [usingId, setUsingId] = useState<number | null>(null);
  const apiStatus = useApiStatusContext();

  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);

  useEffect(() => {
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
    }
  }, [isLoggedIn]);

  const fetchInventory = useCallback(async () => {
    try {
      const res = await itemApi.getInventory();
      const data = res.data?.data;
      if (Array.isArray(data)) {
        setItems(data);
      } else if (data && typeof data === 'object' && 'list' in data) {
        setItems((data as any).list);
      } else {
        setItems([]);
      }
    } catch {
      setItems([]);
    }
  }, []);

  useEffect(() => {
    if (!isLoggedIn) return;
    setLoading(true);
    fetchInventory().finally(() => setLoading(false));
  }, [isLoggedIn, fetchInventory]);

  const handleRefresh = async () => {
    setRefreshing(true);
    await fetchInventory();
    setRefreshing(false);
  };

  const handleUse = async (item: InventoryItem) => {
    if (usingId) return;
    setUsingId(item.item_id);
    try {
      await itemApi.useItem({ item_id: item.item_id, quantity: 1 });
      await fetchInventory();
      apiStatus.showSuccess?.(`Used ${item.name}`);
    } catch {
      apiStatus.showError?.('Failed to use item');
    } finally {
      setUsingId(null);
    }
  };

  if (!isLoggedIn) {
    return (
      <div>
        <Navbar onLoginClick={() => {}} onRegisterClick={() => {}} />
        <div className="pt-32 px-4 flex flex-col items-center justify-center text-center">
          <div className="w-16 h-16 rounded-full bg-[#1a1a2e] flex items-center justify-center mb-4">
            <Package className="w-8 h-8 text-[#8892b0]" />
          </div>
          <p className="text-sm text-[#8892b0]">Please log in to view your backpack</p>
        </div>
      </div>
    );
  }

  return (
    <div>
      <Navbar onLoginClick={() => {}} onRegisterClick={() => {}} />

      <main className="pt-14 px-4 pb-24">
        {/* Header */}
        <div className="flex items-center justify-between mt-3 mb-4">
          <h1 className="text-lg font-bold text-white">Backpack</h1>
          <button
            onClick={handleRefresh}
            disabled={refreshing}
            className="w-8 h-8 rounded-full bg-[#1a1a2e] flex items-center justify-center active:bg-[#1a1a2e]/80"
          >
            <RefreshCw className={`w-4 h-4 text-[#8892b0] ${refreshing ? 'animate-spin' : ''}`} />
          </button>
        </div>

        {/* Content */}
        {loading ? (
          <div className="flex items-center justify-center py-24">
            <Loader2 className="w-6 h-6 text-[#f5a623] animate-spin" />
          </div>
        ) : items.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-24 text-center">
            <div className="w-16 h-16 rounded-full bg-gradient-to-br from-[#f5a623]/20 to-[#e94560]/20 flex items-center justify-center mb-4">
              <Package className="w-8 h-8 text-[#8892b0]" />
            </div>
            <p className="text-sm text-[#8892b0]">No items yet</p>
            <p className="text-[10px] text-[#8892b0]/60 mt-1">Items you earn from activities will appear here</p>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-3">
            {items.map((item) => {
              const cfg = TYPE_CONFIG[item.type] || TYPE_CONFIG[1];
              const TypeIcon = cfg.icon;
              const canUse = item.type !== 3 && item.quantity > 0;

              return (
                <div
                  key={item.id}
                  className="rounded-xl bg-[#1a1a2e]/60 border border-[#f5a623]/10 p-3 flex flex-col"
                >
                  {/* Icon */}
                  <div className="relative w-full aspect-square rounded-lg bg-gradient-to-br from-[#f5a623]/20 to-[#e94560]/20 flex items-center justify-center mb-2.5 overflow-hidden">
                    {item.icon ? (
                      <img
                        src={item.icon}
                        alt={item.name}
                        className="w-12 h-12 object-contain"
                        onError={(e) => {
                          (e.target as HTMLImageElement).style.display = 'none';
                          (e.target as HTMLImageElement).nextElementSibling?.classList.remove('hidden');
                        }}
                      />
                    ) : null}
                    <span className={`text-2xl font-bold ${cfg.color} ${item.icon ? 'hidden' : ''}`}>
                      {item.name.charAt(0).toUpperCase()}
                    </span>
                    {/* Quantity badge */}
                    {item.quantity > 1 && (
                      <span className="absolute top-1 right-1 bg-[#e94560] text-white text-[10px] font-bold min-w-[20px] h-5 rounded-full flex items-center justify-center px-1">
                        {item.quantity}
                      </span>
                    )}
                  </div>

                  {/* Info */}
                  <div className="flex-1 min-w-0">
                    <p className="text-xs font-medium text-[#ccd6f6] truncate">{item.name}</p>
                    <div className="flex items-center gap-1 mt-1">
                      <TypeIcon className="w-3 h-3" style={{ color: cfg.color.replace('text-', '#') }} />
                      <span className={`text-[10px] ${cfg.color}`}>{cfg.label}</span>
                    </div>
                    {item.expire_at && (
                      <p className={`text-[10px] mt-1 ${isExpiringSoon(item.expire_at) ? 'text-yellow-400' : 'text-[#8892b0]'}`}>
                        {isExpiringSoon(item.expire_at) ? 'Expires soon: ' : 'Expires: '}
                        {formatExpiry(item.expire_at)}
                      </p>
                    )}
                  </div>

                  {/* Use button */}
                  {canUse && (
                    <button
                      onClick={() => handleUse(item)}
                      disabled={usingId === item.item_id}
                      className="w-full mt-2.5 bg-[#f5a623] text-[#0a0a1a] text-xs font-medium py-1.5 rounded-lg active:opacity-80 disabled:opacity-50 flex items-center justify-center gap-1"
                    >
                      {usingId === item.item_id ? (
                        <Loader2 className="w-3 h-3 animate-spin" />
                      ) : (
                        <Zap className="w-3 h-3" />
                      )}
                      Use
                    </button>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </main>
    </div>
  );
}
