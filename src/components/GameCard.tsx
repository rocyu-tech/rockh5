'use client';

import { useState } from 'react';
import { Play, Flame, Star, Sparkles, Heart } from 'lucide-react';
import { gameApi, type Game } from '@/lib/api';
import { useAuthStore } from '@/store/auth';
import { toast } from 'sonner';

const gameGradients = [
  'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
  'linear-gradient(135deg, #f093fb 0%, #f5576c 100%)',
  'linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)',
  'linear-gradient(135deg, #43e97b 0%, #38f9d7 100%)',
  'linear-gradient(135deg, #fa709a 0%, #fee140 100%)',
  'linear-gradient(135deg, #a18cd1 0%, #fbc2eb 100%)',
  'linear-gradient(135deg, #fccb90 0%, #d57eeb 100%)',
  'linear-gradient(135deg, #e0c3fc 0%, #8ec5fc 100%)',
  'linear-gradient(135deg, #f5576c 0%, #ff9a9e 100%)',
  'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
  'linear-gradient(135deg, #89f7fe 0%, #66a6ff 100%)',
  'linear-gradient(135deg, #ffecd2 0%, #fcb69f 100%)',
];

interface GameCardProps {
  game: Game;
}

export default function GameCard({ game }: GameCardProps) {
  const [isHovered, setIsHovered] = useState(false);
  const [launching, setLaunching] = useState(false);
  const [isFavorite, setIsFavorite] = useState(!!game.is_favorite);
  const isLoggedIn = useAuthStore(s => s.isLoggedIn);

  const handlePlay = async () => {
    if (launching) return;
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
      return;
    }
    setLaunching(true);
    try {
      const res = await gameApi.launch(game.id);
      const gameUrl = res.data?.data?.launch_url || res.data?.data?.game_url;
      if (gameUrl) {
        window.open(gameUrl, '_blank');
      }
    } catch (err) {
      console.error('[GameCard] launch error:', err);
    } finally {
      setLaunching(false);
    }
  };

  const handleFavorite = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
      return;
    }
    try {
      const res = await gameApi.toggleFavorite(game.id);
      if (res.data?.code === 0) {
        setIsFavorite(res.data.data.is_favorite);
        toast.success(res.data.data.is_favorite ? 'Added to favorites' : 'Removed from favorites');
      }
    } catch {
      toast.error('Failed to update favorite');
    }
  };

  // On mobile, tap toggles overlay; on desktop, hover shows overlay
  const handleToggle = () => {
    setIsHovered((prev) => !prev);
  };

  return (
    <div
      className="group relative rounded-xl overflow-hidden bg-[#1a1a2e] border border-[#f5a623]/10 hover:border-[#f5a623]/30 transition-all duration-300 hover:shadow-lg hover:shadow-[#f5a623]/10 cursor-pointer"
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      onClick={handleToggle}
    >
      {/* Thumbnail */}
      <div
        className="relative aspect-[4/5] overflow-hidden"
        style={{ background: gameGradients[game.id % gameGradients.length] }}
      >
        {/* Pattern overlay */}
        <div className="absolute inset-0 opacity-20">
          <div className="absolute inset-0" style={{
            backgroundImage: `radial-gradient(circle at 30% 40%, rgba(255,255,255,0.1) 0%, transparent 60%),
                             radial-gradient(circle at 70% 60%, rgba(255,255,255,0.05) 0%, transparent 40%)`
          }} />
        </div>

        {/* Game name overlay at bottom */}
        <div className="absolute bottom-0 left-0 right-0 p-3 bg-gradient-to-t from-black/80 to-transparent">
          <h3 className="text-sm font-semibold text-white truncate">{game.name}</h3>
          {game.vendor_name && (
            <span className="text-xs text-white/60">{game.vendor_name}</span>
          )}
        </div>

        {/* Tags */}
        <div className="absolute top-2 left-2 flex gap-1">
          {game.hot && (
            <span className="flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-[#e94560] text-white text-[10px] font-semibold">
              <Flame className="w-3 h-3" /> HOT
            </span>
          )}
          {game.new && (
            <span className="flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-[#4ecdc4] text-[#0a0a1a] text-[10px] font-semibold">
              <Sparkles className="w-3 h-3" /> NEW
            </span>
          )}
        </div>

        {/* Favorite button */}
        <button
          onClick={handleFavorite}
          className="absolute top-2 right-2 w-7 h-7 rounded-full bg-black/40 flex items-center justify-center hover:bg-black/60 transition-colors z-10"
        >
          <Heart
            className={`w-3.5 h-3.5 transition-colors ${isFavorite ? 'text-[#e94560] fill-[#e94560]' : 'text-white/60'}`}
          />
        </button>

        {/* Play overlay - shows on hover (desktop) or tap (mobile) */}
        <div
          className={`absolute inset-0 flex items-center justify-center bg-black/60 backdrop-blur-sm transition-opacity duration-300 ${
            isHovered ? 'opacity-100' : 'opacity-0'
          }`}
          onClick={(e) => e.stopPropagation()}
        >
          <button
            onClick={handlePlay}
            disabled={launching}
            className="w-14 h-14 rounded-full bg-gradient-to-r from-[#f5a623] to-[#e8a910] flex items-center justify-center shadow-lg shadow-[#f5a623]/30 hover:scale-110 active:scale-95 transition-transform disabled:opacity-50"
          >
            {launching ? (
              <div className="w-5 h-5 border-2 border-[#0a0a1a]/30 border-t-[#0a0a1a] rounded-full animate-spin" />
            ) : (
              <Play className="w-5 h-5 text-[#0a0a1a] ml-0.5" fill="#0a0a1a" />
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
