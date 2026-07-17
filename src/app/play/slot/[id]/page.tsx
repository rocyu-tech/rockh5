'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { ArrowLeft, Coins, Loader2 } from 'lucide-react';
import { GameWSClient, getAuthToken } from '@/lib/ws';
import { useAuthStore } from '@/store/auth';
import { useRouter } from 'next/navigation';

export default function SlotGamePage({ params }: { params: Promise<{ id: string }> }) {
  const router = useRouter();
  // Next.js 15: params is now a Promise — unwrap via React.useState + useEffect
  const [gameId, setGameId] = useState<string>('');
  useEffect(() => {
    params.then((p) => setGameId(p.id));
  }, [params]);
  const { assets, fetchAssets } = useAuthStore();
  const wsRef = useRef<GameWSClient | null>(null);

  const [reels, setReels] = useState<number[][]>([]);
  const [spinning, setSpinning] = useState(false);
  const [lastWin, setLastWin] = useState<number | null>(null);
  const [bet, setBet] = useState(1000);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);

  // Connect to WS
  useEffect(() => {
    const token = getAuthToken();
    if (!token) { router.push('/'); return; }

    const ws = new GameWSClient({
      url: `/ws/v1/game/slot`,
      token,
      onOpen: () => setConnected(true),
      onClose: () => setConnected(false),
    });

    // P0: handle server-side error pushes (rate limit, unauthorized, etc.)
    ws.on('error', (_action, data) => {
      const d = data as { message?: string };
      setError(d?.message || 'Server error');
      setSpinning(false);
    });

    // P0: handle balance push (server may push updated balance after settle)
    ws.on('balance', (_action, data) => {
      const d = data as { balance?: number };
      if (typeof d?.balance === 'number') {
        // Refresh from server to keep state authoritative
        fetchAssets();
      }
    });

    wsRef.current = ws;
    ws.connect();

    return () => { ws.close(); };
  }, [router, fetchAssets]);

  const handleSpin = useCallback(async () => {
    if (!wsRef.current || spinning || !connected) return;
    setSpinning(true);
    setError(null);
    setLastWin(null);

    // Animate reels while waiting
    setReels([
      [Math.floor(Math.random() * 10), Math.floor(Math.random() * 10), Math.floor(Math.random() * 10)],
      [Math.floor(Math.random() * 10), Math.floor(Math.random() * 10), Math.floor(Math.random() * 10)],
      [Math.floor(Math.random() * 10), Math.floor(Math.random() * 10), Math.floor(Math.random() * 10)],
      [Math.floor(Math.random() * 10), Math.floor(Math.random() * 10), Math.floor(Math.random() * 10)],
      [Math.floor(Math.random() * 10), Math.floor(Math.random() * 10), Math.floor(Math.random() * 10)],
    ]);

    try {
      const result = await wsRef.current.request('spin', { game_id: gameId, bet }) as {
        reels: number[][];
        win: number;
        paylines?: unknown[];
        scatter_wins?: unknown[];
        free_spins_awarded?: number;
        jackpot_win?: number;
      };

      setReels(result.reels || []);
      setLastWin(result.win || 0);

      if (result.free_spins_awarded && result.free_spins_awarded > 0) {
        setError(`🎉 ${result.free_spins_awarded} Free Spins awarded!`);
      }
      if (result.jackpot_win && result.jackpot_win > 0) {
        setError(`🏆 JACKPOT! Won ${result.jackpot_win}`);
      }

      fetchAssets(); // Refresh balance
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Spin failed');
    } finally {
      setSpinning(false);
    }
  }, [spinning, connected, gameId, bet, fetchAssets]);

  const symbolEmoji = (s: number) => {
    const emojis = ['🍒', '🍋', '🍊', '🍇', '🔔', '⭐', '💎', '7️⃣', '🎰', '💰'];
    return emojis[s % emojis.length];
  };

  return (
    <div className="min-h-screen bg-gradient-to-b from-purple-900 via-purple-800 to-indigo-900 text-white">
      {/* Header */}
      <div className="flex items-center justify-between p-4">
        <button onClick={() => router.back()} className="flex items-center gap-2 text-white/80 hover:text-white">
          <ArrowLeft className="w-5 h-5" /> Back
        </button>
        <div className="flex items-center gap-2 bg-black/30 px-4 py-2 rounded-full">
          <Coins className="w-4 h-4 text-yellow-400" />
          <span className="font-bold">{((assets?.balance || 0) / 100).toFixed(2)}</span>
        </div>
      </div>

      {/* Game Area */}
      <div className="flex flex-col items-center justify-center p-4">
        <h1 className="text-2xl font-bold mb-4">🎰 {gameId}</h1>

        {/* Reels Display */}
        <div className="bg-black/40 rounded-2xl p-6 mb-6 border-2 border-yellow-500/30">
          {reels.length > 0 ? (
            <div className="flex gap-3">
              {reels.map((reel, i) => (
                <div key={i} className="flex flex-col gap-2">
                  {reel.map((symbol, j) => (
                    <div
                      key={j}
                      className={`w-16 h-16 flex items-center justify-center text-3xl bg-white/10 rounded-lg ${spinning ? 'animate-pulse' : ''}`}
                    >
                      {symbolEmoji(symbol)}
                    </div>
                  ))}
                </div>
              ))}
            </div>
          ) : (
            <div className="w-80 h-48 flex items-center justify-center text-white/40">
              Press SPIN to start
            </div>
          )}
        </div>

        {/* Win Display */}
        {lastWin !== null && lastWin > 0 && (
          <div className="text-2xl font-bold text-yellow-400 mb-4 animate-bounce">
            🎉 +{(lastWin / 100).toFixed(2)}
          </div>
        )}

        {/* Error */}
        {error && (
          <div className="text-sm text-red-400 mb-4">{error}</div>
        )}

        {/* Bet Controls */}
        <div className="flex items-center gap-4 mb-6">
          <button
            onClick={() => setBet(Math.max(100, bet - 100))}
            className="w-10 h-10 rounded-full bg-white/10 hover:bg-white/20 font-bold"
          >
            -
          </button>
          <div className="text-center">
            <div className="text-xs text-white/60">BET</div>
            <div className="text-lg font-bold">{(bet / 100).toFixed(2)}</div>
          </div>
          <button
            onClick={() => setBet(bet + 100)}
            className="w-10 h-10 rounded-full bg-white/10 hover:bg-white/20 font-bold"
          >
            +
          </button>
        </div>

        {/* Spin Button */}
        <button
          onClick={handleSpin}
          disabled={spinning || !connected}
          className="px-12 py-4 rounded-full bg-gradient-to-r from-yellow-400 to-orange-500 text-black font-bold text-xl disabled:opacity-50 hover:scale-105 transition-transform"
        >
          {spinning ? <Loader2 className="w-6 h-6 animate-spin mx-auto" /> : 'SPIN'}
        </button>

        {!connected && (
          <div className="mt-4 text-sm text-yellow-400">Connecting...</div>
        )}
      </div>
    </div>
  );
}
