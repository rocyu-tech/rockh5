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

interface BaccaratResult {
  player_cards: number[];
  banker_cards: number[];
  player_total: number;
  banker_total: number;
  winner: string;
  natural: boolean;
  player_pair: boolean;
  banker_pair: boolean;
  bet_type: string;
  bet_amount: number;
  win_amount: number;
}

export default function BaccaratPage() {
  const router = useRouter();
  const { assets, fetchAssets } = useAuthStore();
  const wsRef = useRef<GameWSClient | null>(null);

  const [result, setResult] = useState<BaccaratResult | null>(null);
  const [betting, setBetting] = useState(false);
  const [bet, setBet] = useState(1000);
  const [selectedType, setSelectedType] = useState<string>('');
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    const token = getAuthToken();
    if (!token) { router.push('/'); return; }

    const ws = new GameWSClient({
      url: `/ws/v1/game/baccarat/table`,
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
    setSelectedType(betType);
    setBetting(true);
    setError(null);
    setResult(null);

    try {
      const res = await wsRef.current.request('baccarat_bet', { bet_type: betType, amount: bet }) as BaccaratResult;
      setResult(res);
      fetchAssets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Bet failed');
    } finally {
      setBetting(false);
    }
  }, [betting, connected, bet, fetchAssets]);

  const betOptions = [
    { type: 'player', label: 'Player', payout: '1:1', color: 'bg-blue-600' },
    { type: 'banker', label: 'Banker', payout: '0.95:1', color: 'bg-red-600' },
    { type: 'tie', label: 'Tie', payout: '8:1', color: 'bg-green-600' },
    { type: 'player_pair', label: 'Player Pair', payout: '11:1', color: 'bg-purple-600' },
    { type: 'banker_pair', label: 'Banker Pair', payout: '11:1', color: 'bg-orange-600' },
  ];

  return (
    <div className="min-h-screen bg-gradient-to-b from-green-900 via-green-800 to-emerald-900 text-white">
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
        <h1 className="text-2xl font-bold mb-6">🃏 Baccarat</h1>

        {/* Cards Display */}
        {result && (
          <div className="mb-6 space-y-4">
            {/* Banker */}
            <div className="text-center">
              <div className="text-sm text-white/60 mb-1">Banker ({result.banker_total})</div>
              <div className="flex gap-2 justify-center">
                {result.banker_cards.map((c, i) => (
                  <div key={i} className={`w-14 h-20 bg-white rounded-lg flex items-center justify-center text-lg font-bold ${cardColor(c)}`}>
                    {cardName(c)}
                  </div>
                ))}
              </div>
            </div>
            {/* Player */}
            <div className="text-center">
              <div className="text-sm text-white/60 mb-1">Player ({result.player_total})</div>
              <div className="flex gap-2 justify-center">
                {result.player_cards.map((c, i) => (
                  <div key={i} className={`w-14 h-20 bg-white rounded-lg flex items-center justify-center text-lg font-bold ${cardColor(c)}`}>
                    {cardName(c)}
                  </div>
                ))}
              </div>
            </div>
            {/* Result */}
            <div className="text-center">
              <div className="text-xl font-bold text-yellow-400">
                {result.winner === 'tie' ? '🤝 Tie!' : `🏆 ${result.winner} wins!`}
                {result.natural && ' (Natural)'}
              </div>
              {result.win_amount > 0 && (
                <div className="text-2xl font-bold text-green-400 mt-2">
                  +{(result.win_amount / 100).toFixed(2)}
                </div>
              )}
            </div>
          </div>
        )}

        {error && <div className="text-red-400 mb-4">{error}</div>}
        {betting && <Loader2 className="w-6 h-6 animate-spin mb-4" />}

        {/* Bet Amount */}
        <div className="flex items-center gap-4 mb-6">
          <button onClick={() => setBet(Math.max(100, bet - 100))} className="w-10 h-10 rounded-full bg-white/10 font-bold">-</button>
          <div className="text-center">
            <div className="text-xs text-white/60">BET</div>
            <div className="text-lg font-bold">{(bet / 100).toFixed(2)}</div>
          </div>
          <button onClick={() => setBet(bet + 100)} className="w-10 h-10 rounded-full bg-white/10 font-bold">+</button>
        </div>

        {/* Bet Options */}
        <div className="grid grid-cols-2 gap-3 w-full max-w-md">
          {betOptions.map((opt) => (
            <button
              key={opt.type}
              onClick={() => handleBet(opt.type)}
              disabled={betting || !connected}
              className={`${opt.color} rounded-xl py-4 font-bold disabled:opacity-50 hover:opacity-90 transition-opacity`}
            >
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
