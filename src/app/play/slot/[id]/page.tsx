'use client';

// Slot game page with real scrolling reel animation.
//
// P0-10 FIX: previously the spin just `setReels(Math.random()...)` with
// `animate-pulse` (opacity flicker). Now each reel is a vertical strip
// of N+3 symbols (3 visible + N hidden above/below for the scroll
// illusion). On spin:
//   1. Set the strip to a long random sequence.
//   2. Translate-Y the strip from 0 to (final_position) over a duration
//      with a cubic-bezier deceleration curve.
//   3. Stagger each reel's stop by 150ms so they stop left-to-right.
//   4. When the server result arrives, swap the strip's tail to the
//      actual result symbols so the final visible row matches.

import { useState, useEffect, useRef, useCallback } from 'react';
import { ArrowLeft, Loader2 } from 'lucide-react';
import { GameWSClient, getAuthToken } from '@/lib/ws';
import { useAuthStore } from '@/store/auth';
import { useRouter } from 'next/navigation';
import { fmtMoneyPlain } from '@/lib/money';

const SYMBOLS = ['🍒', '🍋', '🍊', '🍇', '🔔', '⭐', '💎', '7️⃣', '🎰', '💰'];
const VISIBLE_ROWS = 3;
const REEL_COUNT = 5;
const STRIP_LEN = 30; // long enough that the scroll looks continuous
const ROW_HEIGHT = 64; // px — must match `h-16` below

interface ReelState {
  strip: number[];          // full strip of symbol indices
  offset: number;           // current translateY in px
  isSpinning: boolean;
  finalSymbols: number[];  // the 3 visible symbols after stop
}

function randomSymbol(): number {
  return Math.floor(Math.random() * SYMBOLS.length);
}

function makeStrip(): number[] {
  return Array.from({ length: STRIP_LEN }, randomSymbol);
}

function makeInitialReels(): ReelState[] {
  return Array.from({ length: REEL_COUNT }, () => ({
    strip: makeStrip(),
    offset: 0,
    isSpinning: false,
    finalSymbols: [randomSymbol(), randomSymbol(), randomSymbol()],
  }));
}

export default function SlotGamePage({ params }: { params: Promise<{ id: string }> }) {
  const router = useRouter();
  const [gameId, setGameId] = useState<string>('');
  useEffect(() => {
    params.then((p) => setGameId(p.id));
  }, [params]);
  const { assets, fetchAssets } = useAuthStore();
  const wsRef = useRef<GameWSClient | null>(null);

  const [reels, setReels] = useState<ReelState[]>(makeInitialReels);
  const [spinning, setSpinning] = useState(false);
  const [lastWin, setLastWin] = useState<number | null>(null);
  const [bet, setBet] = useState(1000);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    const token = getAuthToken();
    if (!token) { router.push('/'); return; }
    if (!gameId) return;

    const ws = new GameWSClient({
      url: `/ws/v1/game/slot`,
      token,
      onOpen: () => setConnected(true),
      onClose: () => setConnected(false),
    });

    ws.on('error', (_action, data) => {
      const d = data as { message?: string };
      setError(d?.message || 'Server error');
      setSpinning(false);
    });

    ws.on('balance', () => {
      fetchAssets();
    });

    wsRef.current = ws;
    ws.connect();

    return () => { ws.close(); };
  }, [router, fetchAssets, gameId]);

  const handleSpin = useCallback(async () => {
    if (!wsRef.current || spinning || !connected || !gameId) return;
    setSpinning(true);
    setError(null);
    setLastWin(null);

    // Phase 1: start the visual spin animation immediately.
    // Each reel translates from offset 0 to (STRIP_LEN - VISIBLE_ROWS) * ROW_HEIGHT.
    // We re-randomize the strip so the user sees fresh symbols scrolling.
    const newStrips = Array.from({ length: REEL_COUNT }, () => makeStrip());
    setReels((prev) => prev.map((r, i) => ({
      ...r,
      strip: newStrips[i],
      offset: 0,
      isSpinning: true,
    })));

    // Trigger the CSS transition by changing offset after a microtask
    // (so React commits the 0 offset first, then animates to the target).
    setTimeout(() => {
      setReels((prev) => prev.map((r) => ({
        ...r,
        offset: (STRIP_LEN - VISIBLE_ROWS) * ROW_HEIGHT,
      })));
    }, 30);

    try {
      const result = await wsRef.current.request('spin', { game_id: gameId, bet }) as {
        reels: number[][];
        win: number;
        paylines?: unknown[];
        scatter_wins?: unknown[];
        free_spins_awarded?: number;
        jackpot_win?: number;
      };

      // Phase 2: when the result arrives, "snap" each reel's tail to the
      // actual result symbols. We replace the last 3 entries of the strip
      // with the server-provided row, so when the transition ends the
      // visible row matches.
      const finalReels = result.reels || [];
      setReels((prev) => prev.map((r, i) => {
        const newStrip = [...r.strip];
        // finalReels[i] is the 3 visible symbols for reel i (top-to-bottom).
        // Place them at the END of the strip (positions STRIP_LEN-3..STRIP_LEN-1)
        // so they appear when the scroll completes.
        const serverRow = finalReels[i] || [randomSymbol(), randomSymbol(), randomSymbol()];
        for (let j = 0; j < VISIBLE_ROWS; j++) {
          newStrip[STRIP_LEN - VISIBLE_ROWS + j] = serverRow[j] ?? randomSymbol();
        }
        return {
          ...r,
          strip: newStrip,
          finalSymbols: serverRow,
        };
      }));

      setLastWin(result.win || 0);

      if (result.free_spins_awarded && result.free_spins_awarded > 0) {
        setError(`🎉 ${result.free_spins_awarded} Free Spins awarded!`);
      }
      if (result.jackpot_win && result.jackpot_win > 0) {
        setError(`🏆 JACKPOT! Won ${result.jackpot_win}`);
      }

      fetchAssets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Spin failed');
      // Reset reels to idle
      setReels((prev) => prev.map((r) => ({ ...r, offset: 0, isSpinning: false })));
    } finally {
      // Phase 3: after the transition completes (~2s), stop the spinning flag.
      // Stagger each reel so they stop one by one (left to right).
      for (let i = 0; i < REEL_COUNT; i++) {
        setTimeout(() => {
          setReels((prev) => prev.map((r, idx) => idx === i ? { ...r, isSpinning: false } : r));
          if (i === REEL_COUNT - 1) setSpinning(false);
        }, 1800 + i * 200);
      }
    }
  }, [spinning, connected, gameId, bet, fetchAssets]);

  const symbolEmoji = (s: number) => SYMBOLS[s % SYMBOLS.length];

  return (
    <div className="min-h-screen bg-gradient-to-b from-purple-900 via-purple-800 to-indigo-900 text-white">
      {/* Header */}
      <div className="flex items-center justify-between p-4">
        <button
          onClick={() => router.back()}
          aria-label="Back"
          className="flex items-center gap-2 text-white/80 hover:text-white min-w-[44px] min-h-[44px] items-center"
        >
          <ArrowLeft className="w-5 h-5" /> <span className="text-sm hidden sm:inline">Back</span>
        </button>
        <div className="flex items-center gap-2 bg-black/30 px-4 py-2 rounded-full">
          <span className="font-bold">{fmtMoneyPlain(assets?.balance ?? 0)}</span>
        </div>
      </div>

      {/* Game Area */}
      <div className="flex flex-col items-center justify-center p-4">
        <h1 className="text-2xl font-bold mb-4">🎰 {gameId || 'Slot'}</h1>

        {/* Reels Display — overflow-hidden clips the scrolling strip */}
        <div className="bg-black/40 rounded-2xl p-6 mb-6 border-2 border-yellow-500/30">
          <div className="flex gap-3" style={{ height: VISIBLE_ROWS * ROW_HEIGHT, overflow: 'hidden' }}>
            {reels.map((reel, i) => (
              <div
                key={i}
                className="flex flex-col"
                style={{
                  transform: `translateY(-${reel.offset}px)`,
                  transition: spinning
                    ? `transform ${1.8 + i * 0.2}s cubic-bezier(0.25, 0.1, 0.25, 1)`
                    : 'none',
                }}
              >
                {reel.strip.map((symbol, j) => (
                  <div
                    key={j}
                    className="flex items-center justify-center text-3xl bg-white/10 rounded-lg"
                    style={{ width: 64, height: ROW_HEIGHT }}
                  >
                    {symbolEmoji(symbol)}
                  </div>
                ))}
              </div>
            ))}
          </div>
        </div>

        {/* Win Display */}
        {lastWin !== null && lastWin > 0 && (
          <div className="text-2xl font-bold text-yellow-400 mb-4 animate-bounce">
            🎉 +{fmtMoneyPlain(lastWin)}
          </div>
        )}

        {/* Error */}
        {error && (
          <div className="text-sm text-red-400 mb-4 text-center max-w-xs">{error}</div>
        )}

        {/* Bet Controls */}
        <div className="flex items-center gap-4 mb-6">
          <button
            onClick={() => setBet(Math.max(100, bet - 100))}
            aria-label="Decrease bet"
            className="w-11 h-11 rounded-full bg-white/10 hover:bg-white/20 font-bold flex items-center justify-center"
          >
            -
          </button>
          <div className="text-center">
            <div className="text-xs text-white/60">BET</div>
            <div className="text-lg font-bold">{fmtMoneyPlain(bet)}</div>
          </div>
          <button
            onClick={() => setBet(bet + 100)}
            aria-label="Increase bet"
            className="w-11 h-11 rounded-full bg-white/10 hover:bg-white/20 font-bold flex items-center justify-center"
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
