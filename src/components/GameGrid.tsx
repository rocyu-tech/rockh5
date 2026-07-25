'use client';

import { useState, useEffect, useCallback } from 'react';
import { Search, Gamepad2, Loader2, AlertCircle } from 'lucide-react';
import { lobbyApi, type Game } from '@/lib/api';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';
import GameCard from './GameCard';
import { Input } from '@/components/ui/input';

interface GameGridProps {
  categoryId: number;
}

export default function GameGrid({ categoryId }: GameGridProps) {
  const [games, setGames] = useState<Game[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(true);
  const [loadError, setLoadError] = useState('');
  const pageSize = 12;
  const apiStatus = useApiStatusContext();

  const fetchGames = useCallback(async (pageNum: number, catId: number, keyword: string, append: boolean) => {
    setLoading(true);
    setLoadError('');
    try {
      const params: { category_id?: number; keyword?: string; page?: number; page_size?: number } = {
        page: pageNum,
        page_size: pageSize,
      };
      if (catId > 0) params.category_id = catId;
      if (keyword) params.keyword = keyword;

      const res = await lobbyApi.getGames(params);
      const data = res.data;
      const list = data?.games;

      if (list?.length) {
        setGames(append ? (prev) => [...prev, ...list] : list);
        setHasMore(list.length >= pageSize);
      } else {
        setGames(append ? (prev) => prev : []);
        setHasMore(false);
      }
    } catch (err) {
      // BG-7 FIX: show error state instead of falling back to mock data
      const msg = getErrorMessage(err);
      setLoadError(msg);
      apiStatus.markFailed('lobby/games', msg);
      if (!append) setGames([]);
      setHasMore(false);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    setPage(1);
    fetchGames(1, categoryId, searchQuery, false);
  }, [categoryId, searchQuery, fetchGames]);

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
      </div>

      {loadError && games.length === 0 && !loading ? (
        <div className="flex flex-col items-center justify-center py-16 text-[#8892b0]">
          <AlertCircle className="w-12 h-12 mb-4 text-[#e94560] opacity-70" />
          <p className="text-lg font-medium text-[#ccd6f6]">Failed to load games</p>
          <p className="text-sm mt-1 text-[#8892b0]">{loadError}</p>
          <button
            onClick={() => fetchGames(1, categoryId, searchQuery, false)}
            className="mt-4 px-4 py-2 bg-[#f5a623]/20 text-[#f5a623] rounded-lg text-sm hover:bg-[#f5a623]/30 transition-colors"
          >
            Retry
          </button>
        </div>
      ) : games.length === 0 && !loading ? (
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

      {loading && (
        <div className="flex justify-center py-8">
          <Loader2 className="w-6 h-6 text-[#f5a623] animate-spin" />
        </div>
      )}

      {!hasMore && games.length > 0 && (
        <p className="text-center text-sm text-[#8892b0] py-4">
          — No more games —
        </p>
      )}
    </div>
  );
}
