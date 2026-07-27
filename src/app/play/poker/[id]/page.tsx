'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { ArrowLeft, Coins, Loader2 } from 'lucide-react';
import { GameWSClient, getAuthToken } from '@/lib/ws';
import { useAuthStore } from '@/store/auth';
import { useRouter } from 'next/navigation';
import { fmtMoney, fmtMoneyPlain } from '@/lib/money';

const SUITS = ['♠', '♥', '♦', '♣'];
const RANKS = ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K'];

function cardName(card: number): string {
  if (card < 0 || card > 51) return '?';
  const suit = Math.floor(card / 13);
  const rank = card % 13;
  return `${RANKS[rank]}${SUITS[suit]}`;
}

function cardColor(card: number): string {
  if (card < 0 || card > 51) return 'text-gray-400';
  const suit = Math.floor(card / 13);
  return suit === 1 || suit === 2 ? 'text-red-400' : 'text-white';
}

interface PlayerInfo {
  user_id: number;
  seat_index?: number;
  bet: number;
  total_bet?: number;
  folded: boolean;
  all_in: boolean;
  last_action: string;
}

interface WinnerShare {
  user_id: number;
  amount: number;
  hand_rank?: string;
  hand_cards?: number[];
  pot_level?: number;
  is_refund?: boolean;
}

interface GameState {
  phase: string;
  community: number[];
  pot: number;
  current_bet: number;
  current_turn: number;
  players: PlayerInfo[];
}

export default function PokerPage({ params }: { params: Promise<{ id: string }> }) {
  const router = useRouter();
  // Next.js 15: params is now a Promise — unwrap via React.useState + useEffect
  // (use() would be cleaner but requires <Suspense>; useState is simpler).
  const [gameId, setGameId] = useState<string>('');
  useEffect(() => {
    params.then((p) => setGameId(p.id));
  }, [params]);
  const { assets, fetchAssets, user } = useAuthStore();
  const wsRef = useRef<GameWSClient | null>(null);
  const myUserId = user?.id;

  const [connected, setConnected] = useState(false);
  const [matched, setMatched] = useState(false);
  const [gameState, setGameState] = useState<GameState | null>(null);
  const [myHand, setMyHand] = useState<number[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState(false);
  const [raiseAmount, setRaiseAmount] = useState(0);

  // Join table on connect
  useEffect(() => {
    const token = getAuthToken();
    if (!token) { router.push('/'); return; }
    // P0: don't open WS until gameId resolves from the async params Promise
    if (!gameId) return;

    const ws = new GameWSClient({
      url: `/ws/v1/game/poker/table`,
      token,
      onOpen: async () => {
        setConnected(true);
        // Send join request
        try {
          const res = await ws.request('join', { game_id: gameId, min_bet: 1000 }) as { status: string; room_id?: number };
          if (res.status === 'matched') {
            setMatched(true);
          }
        } catch (err) {
          setError(err instanceof Error ? err.message : 'Join failed');
        }
      },
      onClose: () => setConnected(false),
    });

    // Subscribe to push messages
    ws.on('room_ready', (_action, data) => {
      setMatched(true);
    });

    ws.on('game_state', (_action, data) => {
      setGameState(data as GameState);
    });

    // P0 FIX: backend sends `your_hand`, not `deal_hole`. Keep `deal_hole` as
    // a fallback in case the backend re-introduces it.
    const handleHand = (data: unknown) => {
      const d = data as { hand: number[] };
      setMyHand(d.hand || []);
    };
    ws.on('your_hand', (_action, data) => handleHand(data));
    ws.on('deal_hole', (_action, data) => handleHand(data));

    ws.on('round_end', (_action, data) => {
      const d = data as { winner: number; win_amount: number; hand_rank?: string; winners?: WinnerShare[]; rake?: number };
      if (d.winners && d.winners.length > 0) {
        const me = d.winners.find((w) => w.user_id === myUserId);
        if (me) {
          setError(`🏆 You win ${fmtMoneyPlain(me.amount)}! ${me.hand_rank ? `(${me.hand_rank})` : ''}`);
        } else {
          const w0 = d.winners[0];
          setError(`🏆 Player ${w0.user_id} wins ${fmtMoneyPlain(w0.amount)}! ${w0.hand_rank ? `(${w0.hand_rank})` : ''}`);
        }
      } else if (d.winner > 0) {
        const isMe = d.winner === myUserId;
        setError(`🏆 ${isMe ? 'You' : `Player ${d.winner}`} win${isMe ? '' : 's'} ${fmtMoneyPlain(d.win_amount)}! ${d.hand_rank || ''}`);
      }
      fetchAssets();
      // Clear hand after showdown
      setMyHand([]);
    });

    ws.on('player_timeout', (_action, data) => {
      const d = data as { user_id: number };
      setError(`⏰ Player ${d.user_id} timed out (auto-fold)`);
    });

    // P0: handle redirect push (room moved to another game-node)
    ws.on('redirect', (_action, data) => {
      const d = data as { room_id: number; players: number[] };
      console.warn('[poker] server requested redirect to another node:', d);
      setError('Room moved to another server, reconnecting...');
      // The GameWSClient auto-reconnects on close; the server-side should
      // close this connection after sending `redirect`. The next connect
      // attempt will land on the correct node via the gate's sticky LB.
    });

    // P0: handle player_left push (so we can show a status banner)
    ws.on('player_left', (_action, data) => {
      const d = data as { player: number };
      if (d.player !== myUserId) {
        setError(`Player ${d.player} left the table`);
      }
    });

    // P0: handle `error` push from server (rate limit, unauthorized, etc.)
    ws.on('error', (_action, data) => {
      const d = data as { message?: string };
      setError(d?.message || 'Server error');
    });

    wsRef.current = ws;
    ws.connect();
    return () => { ws.close(); };
  }, [router, gameId, fetchAssets, myUserId]);

  const handleAction = useCallback(async (action: string, amount?: number) => {
    if (!wsRef.current || acting) return;
    setActing(true);
    setError(null);
    try {
      await wsRef.current.request(action, amount !== undefined ? { amount } : {});
      fetchAssets();
    } catch (err) {
      setError(err instanceof Error ? err.message : `${action} failed`);
    } finally {
      setActing(false);
    }
  }, [acting, fetchAssets]);

  // P0 FIX: was `Number(localStorage.getItem('rockgame_user_id'))` but that
  // key was never written anywhere — isMyTurn was always false (Number(null)===0).
  // Use the user id from the Zustand auth store instead.
  const isMyTurn = !!myUserId && gameState?.current_turn === myUserId;

  // P0-11: turn countdown timer.
  // Backend enforces a 30s turn limit per player (game_texas.go:160) and
  // auto-folds on expiry. Without a visible countdown, players were being
  // auto-folded without warning — perceived as unfair. Now we tick a
  // 30s timer whenever `isMyTurn` becomes true, and visually warn at 5s.
  const TURN_TIME_SECONDS = 30;
  const [turnSecondsLeft, setTurnSecondsLeft] = useState<number>(TURN_TIME_SECONDS);
  useEffect(() => {
    if (!isMyTurn) {
      setTurnSecondsLeft(TURN_TIME_SECONDS);
      return;
    }
    setTurnSecondsLeft(TURN_TIME_SECONDS);
    const startTs = Date.now();
    const interval = setInterval(() => {
      const elapsed = (Date.now() - startTs) / 1000;
      const left = Math.max(0, TURN_TIME_SECONDS - elapsed);
      setTurnSecondsLeft(left);
      if (left <= 0) clearInterval(interval);
    }, 100);
    return () => clearInterval(interval);
  }, [isMyTurn, gameState?.current_turn]);

  const turnPercent = (turnSecondsLeft / TURN_TIME_SECONDS) * 100;
  const turnCritical = turnSecondsLeft <= 5;

  return (
    <div className="min-h-screen bg-gradient-to-b from-emerald-950 via-green-900 to-emerald-950 text-white">
      <div className="flex items-center justify-between p-4">
        <button onClick={() => router.back()} className="flex items-center gap-2 text-white/80 hover:text-white">
          <ArrowLeft className="w-5 h-5" /> Back
        </button>
        <div className="flex items-center gap-2 bg-black/30 px-4 py-2 rounded-full">
          <Coins className="w-4 h-4 text-yellow-400" />
          <span className="font-bold">{fmtMoneyPlain(assets?.balance ?? 0)}</span>
        </div>
      </div>

      <div className="flex flex-col items-center p-4">
        <h1 className="text-xl font-bold mb-4">🃏 Texas Hold'em</h1>

        {/* Status */}
        {!connected && <div className="text-yellow-400 mb-4">Connecting...</div>}
        {connected && !matched && <div className="text-blue-400 mb-4">Waiting for players...</div>}

        {/* Community Cards */}
        {gameState && (
          <div className="mb-6">
            <div className="text-sm text-white/60 mb-2 text-center">
              Phase: {gameState.phase} | Pot: {fmtMoneyPlain(gameState.pot)}
            </div>
            <div className="flex gap-2 justify-center">
              {[0, 1, 2, 3, 4].map((i) => (
                <div key={i} className="w-14 h-20 bg-black/30 rounded-lg flex items-center justify-center text-lg font-bold border border-white/10">
                  {gameState.community[i] !== undefined ? (
                    <span className={cardColor(gameState.community[i])}>{cardName(gameState.community[i])}</span>
                  ) : (
                    <span className="text-white/20">?</span>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* My Hand */}
        {myHand.length > 0 && (
          <div className="mb-6">
            <div className="text-sm text-white/60 mb-2 text-center">Your Hand</div>
            <div className="flex gap-2 justify-center">
              {myHand.map((c, i) => (
                <div key={i} className="w-14 h-20 bg-white rounded-lg flex items-center justify-center text-lg font-bold">
                  <span className={cardColor(c)}>{cardName(c)}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Error/Info */}
        {error && <div className="text-sm text-yellow-400 mb-4 text-center max-w-xs">{error}</div>}
        {acting && <Loader2 className="w-5 h-5 animate-spin mb-4" />}

        {/* P0-11: Turn countdown timer */}
        {isMyTurn && (
          <div className="mb-4 w-full max-w-xs">
            <div className="flex items-center justify-between text-xs mb-1">
              <span className="text-[#8892b0]">Your turn</span>
              <span className={`font-bold ${turnCritical ? 'text-red-400 animate-pulse' : 'text-yellow-400'}`}>
                {Math.ceil(turnSecondsLeft)}s
              </span>
            </div>
            <div className="h-2 bg-black/30 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-100 ease-linear ${
                  turnCritical ? 'bg-red-500' : 'bg-yellow-400'
                }`}
                style={{ width: `${turnPercent}%` }}
              />
            </div>
            {turnCritical && (
              <p className="text-[10px] text-red-400 text-center mt-1 animate-pulse">
                Auto-fold in {Math.ceil(turnSecondsLeft)}s!
              </p>
            )}
          </div>
        )}

        {/* Actions */}
        {matched && gameState && (
          <div className="flex flex-col items-center gap-3">
            {/* Raise amount input — P1-17: bumped to 44px touch target */}
            <div className="flex items-center gap-2">
              <button
                onClick={() => setRaiseAmount(Math.max(0, raiseAmount - 100))}
                aria-label="Decrease raise amount"
                className="w-11 h-11 rounded bg-white/10 font-bold flex items-center justify-center"
              >
                -
              </button>
              <div className="text-center w-20">
                <div className="text-xs text-white/60">Raise</div>
                <div className="font-bold">{fmtMoneyPlain(raiseAmount)}</div>
              </div>
              <button
                onClick={() => setRaiseAmount(raiseAmount + 100)}
                aria-label="Increase raise amount"
                className="w-11 h-11 rounded bg-white/10 font-bold flex items-center justify-center"
              >
                +
              </button>
            </div>

            {/* Action buttons */}
            <div className="flex gap-2">
              <button onClick={() => handleAction('fold')} disabled={!isMyTurn || acting}
                className="px-6 py-3 rounded-lg bg-red-600 font-bold disabled:opacity-30 hover:bg-red-500">
                Fold
              </button>
              <button onClick={() => handleAction('check')} disabled={!isMyTurn || acting}
                className="px-6 py-3 rounded-lg bg-gray-600 font-bold disabled:opacity-30 hover:bg-gray-500">
                Check
              </button>
              <button onClick={() => handleAction('call')} disabled={!isMyTurn || acting}
                className="px-6 py-3 rounded-lg bg-blue-600 font-bold disabled:opacity-30 hover:bg-blue-500">
                Call
              </button>
              <button onClick={() => handleAction('raise', raiseAmount)} disabled={!isMyTurn || acting || raiseAmount <= 0}
                className="px-6 py-3 rounded-lg bg-green-600 font-bold disabled:opacity-30 hover:bg-green-500">
                Raise
              </button>
              <button onClick={() => handleAction('allin')} disabled={!isMyTurn || acting}
                className="px-6 py-3 rounded-lg bg-yellow-600 text-black font-bold disabled:opacity-30 hover:bg-yellow-500">
                All In
              </button>
            </div>
            {!isMyTurn && <div className="text-xs text-white/40">Waiting for your turn...</div>}
          </div>
        )}
      </div>
    </div>
  );
}
