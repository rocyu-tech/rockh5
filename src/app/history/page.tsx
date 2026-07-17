'use client';

// Game history page — paginated list of the user's past rounds across all
// 4 self-developed games (slot, poker, baccarat, dragon-tiger).
//
// Endpoint: GET /api/v1/game/manage/history?type=all|slot|poker|baccarat|dragon&page=1&page_size=20
// Backend file: internal/handler/game/game_history.go
//
// Filter UI is a horizontal pill bar at the top. Items render as a list of
// cards (mobile-first) with expandable details for slot reels / poker
// community cards / baccarat + dragon-tiger result cards.

import { useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Coins, Loader2, Filter, History as HistoryIcon, ChevronDown, ChevronUp, RefreshCw } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useLocale } from '@/i18n/provider';
import { historyApi } from '@/lib/api';
import { useAuthStore } from '@/store/auth';
import Navbar from '@/components/Navbar';
import { toast } from 'sonner';

type GameType = 'all' | 'slot' | 'poker' | 'baccarat' | 'dragon';

interface HistoryItem {
  id: number;
  game_type: 'slot' | 'poker' | 'baccarat' | 'dragon';
  game_id: string;
  bet_amount: number;
  win_amount: number;
  net: number;
  status: string;
  is_free_spin?: boolean;
  duration?: number;
  player_ids?: number[];
  winner_id?: number;
  hand_rank?: string;
  rake?: number;
  reel_result?: number[][];
  paylines_hit?: Array<{ line: number; symbols: number[]; payout: number }>;
  created_at: string;
}

const GAME_TYPE_META: Record<HistoryItem['game_type'], { icon: string; label: string }> = {
  slot: { icon: '🎰', label: 'history.filterSlot' },
  poker: { icon: '🃏', label: 'history.filterPoker' },
  baccarat: { icon: '🃏', label: 'history.filterBaccarat' },
  dragon: { icon: '🐉', label: 'history.filterDragon' },
};

const SUITS = ['♠', '♥', '♦', '♣'];
const RANKS = ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K'];

function cardName(card: number): string {
  if (card < 0 || card > 51) return '?';
  return `${RANKS[card % 13]}${SUITS[Math.floor(card / 13)]}`;
}

function cardColorClass(card: number): string {
  const suit = Math.floor(card / 13);
  return suit === 1 || suit === 2 ? 'text-red-400' : 'text-white';
}

const SLOT_SYMBOLS = ['🍒', '🍋', '🍊', '🍇', '🔔', '⭐', '💎', '7️⃣', '🎰', '💰'];
function slotEmoji(s: number): string {
  return SLOT_SYMBOLS[s % SLOT_SYMBOLS.length];
}

export default function HistoryPage() {
  const router = useRouter();
  const t = useTranslations();
  const { locale } = useLocale();
  const { isLoggedIn } = useAuthStore();

  const [filter, setFilter] = useState<GameType>('all');
  const [items, setItems] = useState<HistoryItem[]>([]);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expandedId, setExpandedId] = useState<number | null>(null);

  const fetchPage = useCallback(async (pageNum: number, filterType: GameType, append: boolean) => {
    if (append) setLoadingMore(true); else setLoading(true);
    setError(null);
    try {
      const res = await historyApi.list({ type: filterType, page: pageNum, page_size: 20 });
      const data = res.data?.data;
      if (!data) throw new Error('No data');
      setItems(prev => append ? [...prev, ...data.list] : data.list);
      setHasMore(data.has_more);
      setPage(pageNum);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '';
      setError(msg || t('common.error'));
      if (!append) setItems([]);
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  }, [t]);

  // Initial load + on filter change
  useEffect(() => {
    if (!isLoggedIn) return;
    fetchPage(1, filter, false);
  }, [filter, isLoggedIn, fetchPage]);

  // Auth guard
  useEffect(() => {
    if (!isLoggedIn) {
      toast.info(locale === 'zh' ? '请先登录' : 'Please log in first');
      router.push('/');
    }
  }, [isLoggedIn, router, locale]);

  const handleLoadMore = () => {
    if (hasMore && !loadingMore) {
      fetchPage(page + 1, filter, true);
    }
  };

  const handleRefresh = () => {
    fetchPage(1, filter, false);
  };

  const filterOptions: { value: GameType; label: string }[] = [
    { value: 'all', label: t('history.filterAll') },
    { value: 'slot', label: t('history.filterSlot') },
    { value: 'poker', label: t('history.filterPoker') },
    { value: 'baccarat', label: t('history.filterBaccarat') },
    { value: 'dragon', label: t('history.filterDragon') },
  ];

  const fmtMoney = (n: number) => (n / 100).toFixed(2);
  const fmtTime = (s: string) => {
    // Backend already formats as "2006-01-02 15:04:05"; show as-is.
    return s;
  };

  const statusLabel = (s: string) => {
    if (s === 'settled') return t('history.statusSettled');
    if (s === 'cancelled') return t('history.statusCancelled');
    if (s === 'bet') return t('history.statusBet');
    return s;
  };

  const netLabel = (net: number) => {
    if (net > 0) return `+${fmtMoney(net)}`;
    if (net < 0) return `-${fmtMoney(Math.abs(net))}`;
    return fmtMoney(0);
  };

  const netColor = (net: number) => {
    if (net > 0) return 'text-green-400';
    if (net < 0) return 'text-red-400';
    return 'text-[#8892b0]';
  };

  return (
    <div className="min-h-screen bg-[#0a0a1a] text-white">
      <Navbar onLoginClick={() => router.push('/')} onRegisterClick={() => router.push('/')} />

      <main className="pt-14 px-4 pb-20 max-w-2xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-4 mt-4">
          <div className="flex items-center gap-3">
            <HistoryIcon className="w-5 h-5 text-[#f5a623]" />
            <h1 className="text-xl font-bold">{t('history.title')}</h1>
          </div>
          <button
            onClick={handleRefresh}
            disabled={loading}
            className="p-2 rounded-lg bg-[#1a1a2e] hover:bg-[#16213e] transition-colors disabled:opacity-50"
            aria-label="Refresh"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>

        {/* Filter pills */}
        <div className="flex items-center gap-2 mb-4 overflow-x-auto pb-2 -mx-1 px-1 no-scrollbar">
          <Filter className="w-4 h-4 text-[#8892b0] flex-shrink-0" />
          {filterOptions.map((opt) => (
            <button
              key={opt.value}
              onClick={() => setFilter(opt.value)}
              className={`px-3 py-1.5 rounded-full text-xs font-semibold whitespace-nowrap transition-all ${
                filter === opt.value
                  ? 'bg-[#f5a623] text-black'
                  : 'bg-[#1a1a2e] text-[#8892b0] hover:text-white'
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>

        {/* Loading */}
        {loading && (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-6 h-6 animate-spin text-[#f5a623]" />
          </div>
        )}

        {/* Error */}
        {!loading && error && (
          <div className="rounded-lg bg-red-500/10 border border-red-500/30 p-4 text-center">
            <p className="text-sm text-red-300 mb-3">{error}</p>
            <button
              onClick={handleRefresh}
              className="px-4 py-2 rounded-lg bg-[#f5a623] text-black text-sm font-bold hover:opacity-90"
            >
              {t('common.retry')}
            </button>
          </div>
        )}

        {/* Empty */}
        {!loading && !error && items.length === 0 && (
          <div className="rounded-xl border border-white/5 bg-[#1a1a2e] p-8 text-center">
            <HistoryIcon className="w-10 h-10 text-[#8892b0] mx-auto mb-3 opacity-50" />
            <p className="text-sm text-[#8892b0]">{t('history.empty')}</p>
          </div>
        )}

        {/* List */}
        {!loading && !error && items.length > 0 && (
          <div className="space-y-2">
            {items.map((item) => {
              const isExpanded = expandedId === item.id;
              const meta = GAME_TYPE_META[item.game_type];
              const hasDetails =
                (item.game_type === 'slot' && item.reel_result && item.reel_result.length > 0) ||
                (item.game_type !== 'slot' && (item.player_ids || item.winner_id || item.hand_rank));
              return (
                <div
                  key={`${item.game_type}-${item.id}`}
                  className="rounded-xl border border-white/5 bg-[#1a1a2e] overflow-hidden hover:border-white/10 transition-colors"
                >
                  {/* Card header (always visible) */}
                  <button
                    onClick={() => hasDetails && setExpandedId(isExpanded ? null : item.id)}
                    className="w-full p-3 flex items-center justify-between text-left"
                  >
                    <div className="flex items-center gap-3 min-w-0 flex-1">
                      <span className="text-xl flex-shrink-0">{meta.icon}</span>
                      <div className="min-w-0">
                        <p className="text-sm font-semibold truncate">
                          {item.game_id}
                          {item.is_free_spin && (
                            <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded bg-[#f5a623]/20 text-[#f5a623]">FREE</span>
                          )}
                        </p>
                        <p className="text-[10px] text-[#8892b0]">{fmtTime(item.created_at)}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-3 flex-shrink-0">
                      <div className="text-right">
                        <p className="text-[10px] text-[#8892b0]">{t('history.colNet')}</p>
                        <p className={`text-sm font-bold ${netColor(item.net)}`}>
                          {netLabel(item.net)}
                        </p>
                      </div>
                      {hasDetails && (
                        isExpanded
                          ? <ChevronUp className="w-4 h-4 text-[#8892b0]" />
                          : <ChevronDown className="w-4 h-4 text-[#8892b0]" />
                      )}
                    </div>
                  </button>

                  {/* Card body (compact stats row, always visible) */}
                  <div className="px-3 pb-3 grid grid-cols-3 gap-2 text-xs">
                    <div>
                      <p className="text-[10px] text-[#8892b0]">{t('history.colBet')}</p>
                      <p className="font-semibold flex items-center gap-1">
                        <Coins className="w-3 h-3 text-[#f5a623]" />
                        {fmtMoney(item.bet_amount)}
                      </p>
                    </div>
                    <div>
                      <p className="text-[10px] text-[#8892b0]">{t('history.colWin')}</p>
                      <p className="font-semibold text-green-400">{fmtMoney(item.win_amount)}</p>
                    </div>
                    <div>
                      <p className="text-[10px] text-[#8892b0]">{t('history.colStatus')}</p>
                      <p className="font-semibold">{statusLabel(item.status)}</p>
                    </div>
                  </div>

                  {/* Expanded details */}
                  {isExpanded && hasDetails && (
                    <div className="px-3 pb-3 pt-2 border-t border-white/5 space-y-3">
                      {/* Slot reels */}
                      {item.game_type === 'slot' && item.reel_result && item.reel_result.length > 0 && (
                        <div>
                          <p className="text-[10px] text-[#8892b0] uppercase tracking-wider mb-2">Reels</p>
                          <div className="flex gap-2 overflow-x-auto pb-1">
                            {item.reel_result.map((reel, i) => (
                              <div key={i} className="flex flex-col gap-1">
                                {reel.map((sym, j) => (
                                  <div key={j} className="w-10 h-10 flex items-center justify-center text-lg bg-black/30 rounded">
                                    {slotEmoji(sym)}
                                  </div>
                                ))}
                              </div>
                            ))}
                          </div>
                          {item.paylines_hit && item.paylines_hit.length > 0 && (
                            <p className="text-[10px] text-[#8892b0] mt-2">
                              {locale === 'zh' ? '中奖线' : 'Paylines hit'}: {item.paylines_hit.length}
                            </p>
                          )}
                        </div>
                      )}

                      {/* Table game (poker/baccarat/dragon) details */}
                      {item.game_type !== 'slot' && (
                        <div className="grid grid-cols-2 gap-3 text-xs">
                          {item.duration !== undefined && item.duration > 0 && (
                            <div>
                              <p className="text-[10px] text-[#8892b0]">Duration</p>
                              <p className="font-semibold">{item.duration}s</p>
                            </div>
                          )}
                          {item.rake !== undefined && item.rake > 0 && (
                            <div>
                              <p className="text-[10px] text-[#8892b0]">Rake</p>
                              <p className="font-semibold">{fmtMoney(item.rake)}</p>
                            </div>
                          )}
                          {item.winner_id !== undefined && item.winner_id > 0 && (
                            <div>
                              <p className="text-[10px] text-[#8892b0]">Winner</p>
                              <p className="font-semibold">#{item.winner_id}</p>
                            </div>
                          )}
                          {item.hand_rank && (
                            <div>
                              <p className="text-[10px] text-[#8892b0]">Hand</p>
                              <p className="font-semibold">{item.hand_rank}</p>
                            </div>
                          )}
                          {item.player_ids && item.player_ids.length > 0 && (
                            <div className="col-span-2">
                              <p className="text-[10px] text-[#8892b0]">Players</p>
                              <p className="font-semibold text-xs">{item.player_ids.map(id => `#${id}`).join(', ')}</p>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}

            {/* Load more */}
            <div className="pt-2">
              {hasMore ? (
                <button
                  onClick={handleLoadMore}
                  disabled={loadingMore}
                  className="w-full py-3 rounded-lg bg-[#1a1a2e] text-[#8892b0] text-sm font-semibold hover:bg-[#16213e] hover:text-white transition-colors disabled:opacity-50"
                >
                  {loadingMore ? <Loader2 className="w-4 h-4 animate-spin mx-auto" /> : t('history.loadMore')}
                </button>
              ) : (
                <p className="text-center text-xs text-[#8892b0] py-3">{t('history.noMore')}</p>
              )}
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
