'use client';

import { useState, useEffect, useCallback } from 'react';
import { Trophy, Medal, Crown, TrendingUp, Loader2, AlertCircle } from 'lucide-react';
import Navbar from '@/components/Navbar';
import { rankApi, RankItem } from '@/lib/api';
import { toast } from 'sonner';

const RANK_TYPES = [
  { key: 'bet', label: 'Bet Ranking', icon: TrendingUp },
  { key: 'win', label: 'Win Ranking', icon: Trophy },
  { key: 'deposit', label: 'Deposit Ranking', icon: Crown },
];

const PERIODS = [
  { key: 'daily', label: 'Daily' },
  { key: 'weekly', label: 'Weekly' },
  { key: 'monthly', label: 'Monthly' },
];

export default function RankingPage() {
  const [rankType, setRankType] = useState('bet');
  const [period, setPeriod] = useState('daily');
  const [rankList, setRankList] = useState<RankItem[]>([]);
  const [myRank, setMyRank] = useState<RankItem | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchRanks = useCallback(async () => {
    setLoading(true);
    try {
      const [listRes, myRes] = await Promise.all([
        rankApi.getRankList(rankType, period),
        rankApi.getMyRank(rankType),
      ]);
      setRankList(listRes.data?.rank_list || []);
      setMyRank(myRes.data?.my_rank || null);
    } catch {
      toast.error('Failed to load rankings');
    } finally {
      setLoading(false);
    }
  }, [rankType, period]);

  useEffect(() => { fetchRanks(); }, [fetchRanks]);

  const getRankIcon = (rank: number) => {
    if (rank === 1) return <Crown className="w-5 h-5 text-yellow-400" />;
    if (rank === 2) return <Medal className="w-5 h-5 text-gray-300" />;
    if (rank === 3) return <Medal className="w-5 h-5 text-amber-600" />;
    return <span className="text-xs text-[#8892b0] w-5 text-center">{rank}</span>;
  };

  return (
    <div>
      <Navbar
        onLoginClick={() => window.dispatchEvent(new CustomEvent('auth:logout'))}
        onRegisterClick={() => window.dispatchEvent(new CustomEvent('nav:open-register'))}
      />

      <main className="pt-14 px-4">
        <div className="flex items-center gap-2 mb-4">
          <Trophy className="w-5 h-5 text-[#f5a623]" />
          <h1 className="text-lg font-bold text-white">Rankings</h1>
        </div>

        {/* Rank Type Tabs */}
        <div className="flex gap-2 mb-3 overflow-x-auto no-scrollbar">
          {RANK_TYPES.map(rt => {
            const Icon = rt.icon;
            return (
              <button
                key={rt.key}
                onClick={() => setRankType(rt.key)}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium whitespace-nowrap transition-all ${
                  rankType === rt.key
                    ? 'bg-[#f5a623] text-black'
                    : 'bg-[#1e293b] text-[#8892b0] hover:bg-[#2d3a5c]'
                }`}
              >
                <Icon className="w-3.5 h-3.5" />
                {rt.label}
              </button>
            );
          })}
        </div>

        {/* Period Tabs */}
        <div className="flex gap-2 mb-4">
          {PERIODS.map(p => (
            <button
              key={p.key}
              onClick={() => setPeriod(p.key)}
              className={`px-3 py-1 rounded-full text-[11px] font-medium transition-all ${
                period === p.key
                  ? 'bg-[#f5a623]/20 text-[#f5a623] border border-[#f5a623]/40'
                  : 'bg-[#1e293b] text-[#8892b0]'
              }`}
            >
              {p.label}
            </button>
          ))}
        </div>

        {/* Top 3 Podium */}
        {!loading && rankList.length >= 3 && (
          <div className="flex items-end justify-center gap-3 mb-6 px-4">
            {[rankList[1], rankList[0], rankList[2]].map((player, idx) => {
              const heights = ['h-16', 'h-24', 'h-12'];
              const medals = ['🥈', '🥇', '🥉'];
              return (
                <div key={player.user_id} className="flex flex-col items-center flex-1">
                  <div className="w-10 h-10 rounded-full bg-[#1e293b] border-2 border-[#f5a623] overflow-hidden mb-1">
                    {player.avatar ? (
                      <img src={player.avatar} alt="" className="w-full h-full object-cover" />
                    ) : (
                      <div className="w-full h-full flex items-center justify-center text-lg">👤</div>
                    )}
                  </div>
                  <span className="text-[10px] text-white font-medium truncate max-w-full">{player.nickname}</span>
                  <span className="text-[9px] text-[#f5a623]">{player.total_amount.toLocaleString()}</span>
                  <div className={`w-full ${heights[idx]} bg-gradient-to-t from-[#f5a623]/20 to-[#f5a623]/5 rounded-t-lg flex items-start justify-center pt-2`}>
                    <span className="text-2xl">{medals[idx]}</span>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* My Rank */}
        {myRank && (
          <div className="bg-[#f5a623]/10 border border-[#f5a623]/30 rounded-xl p-3 mb-4">
            <div className="flex items-center gap-3">
              {getRankIcon(myRank.rank)}
              <div className="w-8 h-8 rounded-full bg-[#1e293b] overflow-hidden">
                {myRank.avatar ? (
                  <img src={myRank.avatar} alt="" className="w-full h-full object-cover" />
                ) : (
                  <div className="w-full h-full flex items-center justify-center text-sm">👤</div>
                )}
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-xs text-white font-medium truncate">{myRank.nickname} (You)</p>
                <p className="text-[10px] text-[#f5a623]">VIP {myRank.vip_level}</p>
              </div>
              <span className="text-sm font-bold text-[#f5a623]">{myRank.total_amount.toLocaleString()}</span>
            </div>
          </div>
        )}

        {/* Rank List */}
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-6 h-6 text-[#f5a623] animate-spin" />
          </div>
        ) : rankList.length === 0 ? (
          <div className="flex flex-col items-center py-12 text-[#8892b0]">
            <AlertCircle className="w-8 h-8 mb-2" />
            <p className="text-sm">No ranking data</p>
          </div>
        ) : (
          <div className="space-y-1.5">
            {rankList.map((player, idx) => (
              <div
                key={player.user_id}
                className={`flex items-center gap-3 p-2.5 rounded-xl transition-all ${
                  player.user_id === myRank?.user_id
                    ? 'bg-[#f5a623]/10 border border-[#f5a623]/30'
                    : 'bg-[#0d1117] border border-[#1e293b]'
                }`}
              >
                <div className="w-6 flex-shrink-0 flex justify-center">
                  {getRankIcon(player.rank || idx + 1)}
                </div>
                <div className="w-8 h-8 rounded-full bg-[#1e293b] overflow-hidden flex-shrink-0">
                  {player.avatar ? (
                    <img src={player.avatar} alt="" className="w-full h-full object-cover" />
                  ) : (
                    <div className="w-full h-full flex items-center justify-center text-sm">👤</div>
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-xs text-white font-medium truncate">{player.nickname}</p>
                  <p className="text-[10px] text-[#8892b0]">VIP {player.vip_level}</p>
                </div>
                <span className="text-sm font-bold text-[#f5a623] flex-shrink-0">
                  {player.total_amount.toLocaleString()}
                </span>
              </div>
            ))}
          </div>
        )}
      </main>
    </div>
  );
}
