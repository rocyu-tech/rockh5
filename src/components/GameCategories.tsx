'use client';

import { useState, useEffect } from 'react';
import { lobbyApi, type Category } from '@/lib/api';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';

const defaultCategories: Category[] = [
  { id: 0, name: 'All Games', sort_order: 0 },
  { id: 1, name: 'Slots', sort_order: 1 },
  { id: 2, name: 'Live Casino', sort_order: 2 },
  { id: 3, name: 'Sports', sort_order: 3 },
  { id: 4, name: 'Fishing', sort_order: 4 },
  { id: 5, name: 'Table Games', sort_order: 5 },
  { id: 6, name: 'Crash', sort_order: 6 },
  { id: 7, name: 'Poker', sort_order: 7 },
  { id: 8, name: 'Lottery', sort_order: 8 },
];

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
  const [categories, setCategories] = useState<Category[]>(defaultCategories);
  const apiStatus = useApiStatusContext();

  useEffect(() => {
    lobbyApi.getCategories().then((res) => {
      if (res.data?.length) {
        setCategories([{ id: 0, name: 'All Games', sort_order: 0 }, ...res.data]);
      }
    }).catch((err) => {
      apiStatus.markFailed('lobby/categories', getErrorMessage(err));
    });
  }, []);

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
