'use client';

import { useState, useEffect, useCallback } from 'react';
import { Search, Loader2, Gamepad2 } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { lobbyApi, type Game } from '@/lib/api';
import GameCard from './GameCard';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';
import DemoBadge from '@/components/DemoBadge';

// Mock games data
const mockGames: Game[] = Array.from({ length: 24 }, (_, i) => ({
  id: i + 1,
  name: [
    'Fortune Tiger', 'Sweet Bonanza', 'Starlight Princess', 'Gates of Olympus',
    'Wild West Gold', 'Book of Dead', 'Big Bass Bonanza', 'Wolf Gold',
    'Razor Shark', 'Money Train', 'Fire Joker', 'Reactoonz',
    'Gonzo\'s Quest', 'Starburst', 'Mega Moolah', 'Cleopatra',
    'Blackjack Live', 'Roulette VIP', 'Baccarat Pro', 'Poker Stars',
    'Crash X', 'Aviator', 'Spaceman', 'JetX',
  ][i],
  vendor_id: (i % 5) + 1,
  vendor_name: ['PG Soft', 'Pragmatic Play', 'Evolution', 'NetEnt', 'Microgaming'][i % 5],
  category_id: (i % 6) + 1,
  category_name: ['Slots', 'Live Casino', 'Table Games', 'Crash', 'Fishing', 'Sports'][i % 6],
  status: 1,
  hot: i < 6,
  new: i >= 18,
  tag: i < 4 ? 'popular' : undefined,
}));

interface GameGridProps {
  categoryId: number;
}

export default function GameGrid({ categoryId }: GameGridProps) {
  const [games, setGames] = useState<Game[]>(mockGames);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(true);
  const [usingDemo, setUsingDemo] = useState(false);
  const pageSize = 12;
  const apiStatus = useApiStatusContext();

  const fetchGames = useCallback(async (pageNum: number, catId: number, keyword: string, append: boolean) => {
    setLoading(true);
    try {
      const params: { category_id?: number; keyword?: string; page?: number; page_size?: number } = {
        page: pageNum,
        page_size: pageSize,
      };
      if (catId > 0) params.category_id = catId;
      if (keyword) params.keyword = keyword;

      const res = await lobbyApi.getGames(params);
      const data = res.data.data;

      if (data?.list?.length) {
        setGames(append ? (prev) => [...prev, ...data.list] : data.list);
        setHasMore(data.list.length >= pageSize);
      } else {
        // Use mock data filtered
        let filtered = mockGames;
        if (catId > 0) filtered = filtered.filter((g) => g.category_id === catId);
        if (keyword) filtered = filtered.filter((g) => g.name.toLowerCase().includes(keyword.toLowerCase()));
        const start = (pageNum - 1) * pageSize;
        const end = start + pageSize;
        const pageGames = filtered.slice(start, end);
        setGames(append ? (prev) => [...prev, ...pageGames] : pageGames);
        setHasMore(end < filtered.length);
      }
    } catch (err) {
      // Fallback to mock data
      setUsingDemo(true);
      apiStatus.markFailed('lobby/games', getErrorMessage(err));
      let filtered = mockGames;
      if (catId > 0) filtered = filtered.filter((g) => g.category_id === catId);
      if (keyword) filtered = filtered.filter((g) => g.name.toLowerCase().includes(keyword.toLowerCase()));
      const start = (pageNum - 1) * pageSize;
      const end = start + pageSize;
      const pageGames = filtered.slice(start, end);
      setGames(append ? (prev) => [...prev, ...pageGames] : pageGames);
      setHasMore(end < filtered.length);
    } finally {
      setLoading(false);
    }
  }, []);

  // Fetch on category/search change
  useEffect(() => {
    setPage(1);
    fetchGames(1, categoryId, searchQuery, false);
  }, [categoryId, searchQuery, fetchGames]);

  // Infinite scroll
  useEffect(() => {
    const handleScroll = () => {
      if (loading || !hasMore) return;
      const scrollHeight = document.documentElement.scrollHeight;
      const scrollTop = document.documentElement.scrollTop;
      const clientHeight = document.documentElement.clientHeight;
      if (scrollHeight - scrollTop - clientHeight < 300) {
        const nextPage = page + 1;
        setPage(nextPage);
        fetchGames(nextPage, categoryId, searchQuery, true);
      }
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, [loading, hasMore, page, categoryId, searchQuery, fetchGames]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    fetchGames(1, categoryId, searchQuery, false);
  };

  return (
    <div className="w-full space-y-4">
      {/* Search bar + demo badge */}
      <div className="flex items-center gap-3">
      <form onSubmit={handleSearch} className="relative flex-1 max-w-md mx-auto lg:mx-0">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#8892b0]" />
        <Input
          type="text"
          placeholder="Search games..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="pl-10 pr-4 py-2.5 bg-[#16213e] border-[#f5a623]/20 text-[#ccd6f6] placeholder-[#8892b0] rounded-xl focus:border-[#f5a623]/50 focus:ring-[#f5a623]/20"
        />
      </form>
      {usingDemo && <DemoBadge show label="Demo Games" />}
      </div>

      {/* Game grid */}
      {games.length === 0 && !loading ? (
        <div className="flex flex-col items-center justify-center py-16 text-[#8892b0]">
          <Gamepad2 className="w-12 h-12 mb-4 opacity-50" />
          <p className="text-lg font-medium">No games found</p>
          <p className="text-sm mt-1">Try a different search or category</p>
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3 md:gap-4">
          {games.map((game) => (
            <GameCard key={game.id} game={game} />
          ))}
        </div>
      )}

      {/* Loading indicator */}
      {loading && (
        <div className="flex justify-center py-8">
          <Loader2 className="w-6 h-6 text-[#f5a623] animate-spin" />
        </div>
      )}

      {/* No more results */}
      {!hasMore && games.length > 0 && (
        <p className="text-center text-sm text-[#8892b0] py-4">
          — No more games —
        </p>
      )}
    </div>
  );
}
