'use client';

import { useState, useEffect, useCallback } from 'react';
import { type Category } from '@/lib/api';
import { lobbyRpc } from '@/lib/rpc';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';

// Synthetic UI convention — always prepended, never from backend.
const ALL_GAMES: Category = { id: 0, name: 'All Games', sort_order: 0 };

const categoryIcons: Record<string, string> = {
  'All Games': '🎰',
  'Slots': '💎',
  'Live Casino': '🃏',
  'Sports': '⚽',
  'Fishing': '🐟',
  'Table Games': '🎲',
  'Crash': '🚀',
  'Poker': '🂡',
  'Lottery': '🎯',
};

interface GameCategoriesProps {
  activeCategory: number;
  onCategoryChange: (categoryId: number) => void;
}

export default function GameCategories({ activeCategory, onCategoryChange }: GameCategoriesProps) {
  const [categories, setCategories] = useState<Category[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const apiStatus = useApiStatusContext();

  const loadCategories = useCallback(() => {
    setLoading(true);
    setError(null);
    lobbyRpc.getCategories().then((res) => {
      const list = res?.categories;
      if (list?.length) {
        setCategories([ALL_GAMES, ...list]);
      }
      apiStatus.markSuccess('lobby/categories');
    }).catch((err) => {
      setError(getErrorMessage(err));
      apiStatus.markFailed('lobby/categories', getErrorMessage(err));
    }).finally(() => setLoading(false));
  }, [apiStatus]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    lobbyRpc.getCategories().then((res) => {
      if (cancelled) return;
      const list = res?.categories;
      if (list?.length) setCategories([ALL_GAMES, ...list]);
      apiStatus.markSuccess('lobby/categories');
    }).catch((err) => {
      if (cancelled) return;
      setError(getErrorMessage(err));
      apiStatus.markFailed('lobby/categories', getErrorMessage(err));
    }).finally(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, [apiStatus]);

  // Loading state — skeleton, no fake data
  if (loading) {
    return (
      <div className="w-full">
        <div className="flex items-center gap-2 overflow-x-auto hide-scrollbar pb-2 px-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="h-10 w-24 rounded-xl bg-[#1a1a2e]/80 animate-pulse shrink-0" />
          ))}
        </div>
      </div>
    );
  }

  // Error state — explicit message + retry, no silent fallback
  if (error) {
    return (
      <div className="w-full px-2 pb-2">
        <div className="flex items-center gap-2 rounded-xl border border-red-500/20 bg-red-500/5 px-4 py-2.5 text-sm text-red-300">
          <span>⚠</span>
          <span className="flex-1">Categories unavailable — {error}</span>
          <button onClick={loadCategories} className="text-xs underline hover:text-red-200">
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="w-full">
      {/* Desktop scrollable tabs */}
      <div className="hidden sm:flex items-center gap-2 overflow-x-auto hide-scrollbar pb-2 px-2">
        {categories.map((cat) => (
          <button
            key={cat.id}
            onClick={() => onCategoryChange(cat.id)}
            className={`flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-medium whitespace-nowrap transition-all duration-200 ${
              activeCategory === cat.id
                ? 'bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] shadow-lg shadow-[#f5a623]/20'
                : 'bg-[#1a1a2e]/80 text-[#8892b0] hover:text-[#e0e0e0] hover:bg-[#16213e] border border-[#f5a623]/10'
            }`}
          >
            <span>{categoryIcons[cat.name] || '🎮'}</span>
            {cat.name}
            {cat.game_count !== undefined && cat.id !== 0 && (
              <span className="text-xs opacity-70">({cat.game_count})</span>
            )}
          </button>
        ))}
      </div>

      {/* Mobile horizontal scroll */}
      <div className="flex sm:hidden gap-2 overflow-x-auto hide-scrollbar pb-2 px-2">
        {categories.map((cat) => (
          <button
            key={cat.id}
            onClick={() => onCategoryChange(cat.id)}
            className={`flex items-center gap-1.5 px-4 py-2.5 rounded-xl text-xs font-medium whitespace-nowrap transition-all duration-200 ${
              activeCategory === cat.id
                ? 'bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] shadow-lg shadow-[#f5a623]/20'
                : 'bg-[#1a1a2e]/80 text-[#8892b0] hover:text-[#e0e0e0] border border-[#f5a623]/10'
            }`}
          >
            <span className="text-sm">{categoryIcons[cat.name] || '🎮'}</span>
            {cat.name}
          </button>
        ))}
      </div>
    </div>
  );
}
