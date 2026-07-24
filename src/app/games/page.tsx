'use client';

import { useState, useEffect, useCallback } from 'react';
import { Gamepad2, Clock, Loader2 } from 'lucide-react';
import Navbar from '@/components/Navbar';
import GameCategories from '@/components/GameCategories';
import GameGrid from '@/components/GameGrid';
import GameCard from '@/components/GameCard';
import { gameApi, Game } from '@/lib/api';
import { useAuthStore } from '@/store/auth';

export default function GamesPage() {
  const [activeCategory, setActiveCategory] = useState(0);
  const [recentGames, setRecentGames] = useState<Game[]>([]);
  const [loadingRecent, setLoadingRecent] = useState(false);
  const isLoggedIn = useAuthStore(s => s.isLoggedIn);

  const fetchRecent = useCallback(async () => {
    if (!isLoggedIn) return;
    setLoadingRecent(true);
    try {
      const res = await gameApi.getRecentGames();
      setRecentGames(res.data || []);
    } catch { /* ignore */ }
    finally { setLoadingRecent(false); }
  }, [isLoggedIn]);

  useEffect(() => { fetchRecent(); }, [fetchRecent]);

  return (
    <div>
      <Navbar
        onLoginClick={() => window.dispatchEvent(new CustomEvent('auth:logout'))}
        onRegisterClick={() => window.dispatchEvent(new CustomEvent('nav:open-register'))}
      />

      <main className="pt-14 px-4">
        {/* Page header */}
        <div className="flex items-center gap-2 mb-4">
          <Gamepad2 className="w-5 h-5 text-[#f5a623]" />
          <h1 className="text-lg font-bold text-white">Games</h1>
        </div>

        {/* Recent Games */}
        {isLoggedIn && recentGames.length > 0 && (
          <div className="mb-4">
            <div className="flex items-center gap-2 mb-2">
              <Clock className="w-4 h-4 text-[#8892b0]" />
              <h2 className="text-xs font-medium text-[#8892b0]">Recently Played</h2>
            </div>
            <div className="flex gap-3 overflow-x-auto no-scrollbar pb-2">
              {recentGames.slice(0, 10).map(game => (
                <div key={game.id} className="w-24 flex-shrink-0">
                  <GameCard game={game} />
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Categories */}
        <GameCategories
          activeCategory={activeCategory}
          onCategoryChange={setActiveCategory}
        />

        {/* Game Grid */}
        <div className="mt-3">
          <GameGrid categoryId={activeCategory} />
        </div>
      </main>
    </div>
  );
}
