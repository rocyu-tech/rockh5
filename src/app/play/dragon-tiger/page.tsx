'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { ArrowLeft, Coins, Loader2 } from 'lucide-react';
import { GameWSClient, getAuthToken } from '@/lib/ws';
import { useAuthStore } from '@/store/auth';
import { useRouter } from 'next/navigation';

const SUITS = ['♠', '♥', '♦', '♣'];
const RANKS = ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K'];

function cardName(card: number): string {
  const suit = Math.floor(card / 13);
  const rank = card % 13;
  return `${RANKS[rank]}${SUITS[suit]}`;
}

function cardColor(card: number): string {
  const suit = Math.floor(card / 13);
  return suit === 1 || suit === 2 ? 'text-red-400' : 'text-white';
}

interface DragonTigerResult {
  dragon_card: number;
  tiger_card: number;
  dragon_value: number;
  tiger_value: number;
  winner: string;
  suited_tie: boolean;
  bet_type: string;
  bet_amount: number;
  win_amount: number;
}

export default function DragonTigerPage() {
  const router = useRouter();
  const { assets, fetchAssets } = useAuthStore();
  const wsRef = useRef<GameWSClient | null>(null);

  const [result, setResult] = useState<DragonTigerResult | null>(null);
  const [betting, setBetting] = useState(false);
  const [bet, setBet] = useState(1000);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    const token = getAuthToken();
    if (!token) { router.push('/'); return; }
    const ws = new GameWSClient({
      url: `/ws/v1/game/dragon/table`,
      token,
      onOpen: () => setConnected(true),
      onClose: () => setConnected(false),
    });

    // P0: handle server-side error pushes (rate limit, unauthorized, etc.)
    ws.on('error', (_action, data) => {
      const d = data as { message?: string };
      setError(d?.message || 'Server error');
      setBetting(false);
    });

    wsRef.current = ws;
    ws.connect();
    return () => { ws.close(); };
  }, [router]);

  const handleBet = useCallback(async (betType: string) => {
    if (!wsRef.current || betting || !connected) return;
    setBetting(true);
    setError(null);
    setResult(null);
    try {
      const res = await wsRef.current.request('dragon_tiger_bet', { bet_type: betType, amount: bet }) as DragonTigerResult;
      setResult(res);
      fetchAssets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Bet failed');
    } finally {
      setBetting(false);
    }
  }, [betting, connected, bet, fetchAssets]);

  const betOptions = [
    { type: 'dragon', label: '🐉 Dragon', payout: '1:1', color: 'bg-red-600' },
    { type: 'tiger', label: '🐯 Tiger', payout: '1:1', color: 'bg-orange-600' },
    { type: 'tie', label: '🤝 Tie', payout: '8:1', color: 'bg-green-600' },
    { type: 'suited_tie', label: '✨ Suited Tie', payout: '50:1', color: 'bg-purple-600' },
  ];

  return (
    <div className="min-h-screen bg-gradient-to-b from-red-950 via-orange-900 to-amber-900 text-white">
      <div className="flex items-center justify-between p-4">
        <button onClick={() => router.back()} className="flex items-center gap-2 text-white/80 hover:text-white">
          <ArrowLeft className="w-5 h-5" /> Back
        </button>
        <div className="flex items-center gap-2 bg-black/30 px-4 py-2 rounded-full">
          <Coins className="w-4 h-4 text-yellow-400" />
          <span className="font-bold">{((assets?.balance || 0) / 100).toFixed(2)}</span>
        </div>
      </div>

      <div className="flex flex-col items-center p-4">
        <h1 className="text-2xl font-bold mb-6">🐉 Dragon Tiger 🐯</h1>

        {result && (
          <div className="mb-6 space-y-4">
            <div className="flex gap-8 justify-center">
              {/* Dragon */}
              <div className="text-center">
                <div className="text-sm text-white/60 mb-1">🐉 Dragon ({result.dragon_value})</div>
                <div className={`w-16 h-24 bg-white rounded-lg flex items-center justify-center text-xl font-bold ${cardColor(result.dragon_card)}`}>
                  {cardName(result.dragon_card)}
                </div>
              </div>
              {/* Tiger */}
              <div className="text-center">
                <div className="text-sm text-white/60 mb-1">🐯 Tiger ({result.tiger_value})</div>
                <div className={`w-16 h-24 bg-white rounded-lg flex items-center justify-center text-xl font-bold ${cardColor(result.tiger_card)}`}>
                  {cardName(result.tiger_card)}
                </div>
              </div>
            </div>
            <div className="text-center">
              <div className="text-xl font-bold text-yellow-400">
                {result.winner === 'tie' ? `🤝 Tie!${result.suited_tie ? ' (Suited!)' : ''}` : `🏆 ${result.winner} wins!`}
              </div>
              {result.win_amount > 0 && (
                <div className="text-2xl font-bold text-green-400 mt-2">+{(result.win_amount / 100).toFixed(2)}</div>
              )}
            </div>
          </div>
        )}

        {error && <div className="text-red-400 mb-4">{error}</div>}
        {betting && <Loader2 className="w-6 h-6 animate-spin mb-4" />}

        <div className="flex items-center gap-4 mb-6">
          <button onClick={() => setBet(Math.max(100, bet - 100))} className="w-10 h-10 rounded-full bg-white/10 font-bold">-</button>
          <div className="text-center">
            <div className="text-xs text-white/60">BET</div>
            <div className="text-lg font-bold">{(bet / 100).toFixed(2)}</div>
          </div>
          <button onClick={() => setBet(bet + 100)} className="w-10 h-10 rounded-full bg-white/10 font-bold">+</button>
        </div>

        <div className="grid grid-cols-2 gap-3 w-full max-w-md">
          {betOptions.map((opt) => (
            <button key={opt.type} onClick={() => handleBet(opt.type)} disabled={betting || !connected}
              className={`${opt.color} rounded-xl py-4 font-bold disabled:opacity-50 hover:opacity-90`}>
              <div>{opt.label}</div>
              <div className="text-xs opacity-80">{opt.payout}</div>
            </button>
          ))}
        </div>

        {!connected && <div className="mt-4 text-sm text-yellow-400">Connecting...</div>}
      </div>
    </div>
  );
}
