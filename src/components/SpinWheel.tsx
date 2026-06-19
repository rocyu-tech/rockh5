'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { useAuthStore } from '@/store/auth';
import { wheelApi, type WheelConfig, type WheelState, type SpinResult } from '@/lib/api';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Loader2, RotateCw, Gift, Zap, Clock, History, X } from 'lucide-react';

interface SpinWheelProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const RARITY_COLORS: Record<string, string> = {
  common: '#8892b0',
  rare: '#4ecdc4',
  epic: '#a855f7',
  legendary: '#f5a623',
};

const RARITY_BG: Record<string, string> = {
  common: 'from-[#1a1a2e] to-[#16213e]',
  rare: 'from-[#0d3b3a] to-[#1a2e3e]',
  epic: 'from-[#2d1b4e] to-[#1a1a2e]',
  legendary: 'from-[#3d2b0e] to-[#1a1a2e]',
};

const PRIZE_TYPE_ICONS: Record<string, string> = {
  bonus: '$',
  coin: '\u25CF',
  item: '\u2605',
  empty: '\u2014',
};

export default function SpinWheel({ open, onOpenChange }: SpinWheelProps) {
  const { isLoggedIn } = useAuthStore();
  const [config, setConfig] = useState<WheelConfig | null>(null);
  const [state, setState] = useState<WheelState | null>(null);
  const [spinning, setSpinning] = useState(false);
  const [spinResult, setSpinResult] = useState<SpinResult | null>(null);
  const [showResult, setShowResult] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [rotation, setRotation] = useState(0);
  const wheelRef = useRef<HTMLDivElement>(null);

  const fetchData = useCallback(async () => {
    if (!isLoggedIn) return;
    setLoading(true);
    setError('');
    try {
      const [configRes, stateRes] = await Promise.all([
        wheelApi.getConfig(),
        wheelApi.getState(),
      ]);
      setConfig(configRes.data?.data ?? null);
      setState(stateRes.data?.data ?? null);
    } catch (err: unknown) {
      console.error('[SpinWheel] fetch error:', err);
      setError('Failed to load wheel data');
    } finally {
      setLoading(false);
    }
  }, [isLoggedIn]);

  useEffect(() => {
    if (open && isLoggedIn) {
      fetchData();
      setSpinResult(null);
      setShowResult(false);
      setRotation(0);
    }
  }, [open, isLoggedIn, fetchData]);

  const handleSpin = async (useFree: boolean = true) => {
    if (spinning || !state) return;
    if (state.cooldown_remaining > 0) {
      setError(`Please wait ${state.cooldown_remaining}s`);
      return;
    }
    if (useFree && state.remaining_free <= 0 && !state.can_afford_paid) {
      setError('No spins available');
      return;
    }

    setSpinning(true);
    setError('');
    setShowResult(false);

    try {
      const res = await wheelApi.spin(useFree || undefined);
      const result = res.data?.data;
      if (result) {
        // Calculate rotation: multiple full rotations + land on prize index
        const prizes = config?.prizes ?? [];
        const prizeCount = prizes.length;
        const anglePerPrize = 360 / prizeCount;
        const targetAngle = 360 - (result.prize_index * anglePerPrize + anglePerPrize / 2);
        const fullSpins = 5 + Math.floor(Math.random() * 3); // 5-7 full rotations
        const newRotation = rotation + fullSpins * 360 + targetAngle - (rotation % 360);

        setRotation(newRotation);
        setSpinResult(result);

        // Show result after animation ends
        setTimeout(() => {
          setShowResult(true);
          setSpinning(false);
          // Refresh state
          wheelApi.getState().then(s => setState(s.data?.data ?? null));
        }, 4500);
      }
    } catch (err: unknown) {
      setSpinning(false);
      const msg = err instanceof Error ? err.message : 'Spin failed';
      setError(msg);
    }
  };

  const prizes = config?.prizes ?? [];
  const prizeCount = prizes.length;
  const anglePerPrize = prizeCount > 0 ? 360 / prizeCount : 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg bg-[#0a0a1a] border-[#f5a623]/20 text-[#ccd6f6] p-0 overflow-hidden max-h-[90vh]">
        <DialogHeader className="sr-only">
          <DialogTitle>Lucky Wheel</DialogTitle>
          <DialogDescription>Spin the wheel to win prizes</DialogDescription>
        </DialogHeader>

        {/* Close button */}
        <button
          onClick={() => onOpenChange(false)}
          className="absolute top-3 right-3 z-50 w-8 h-8 rounded-full bg-white/10 hover:bg-white/20 flex items-center justify-center transition-colors"
        >
          <X className="w-4 h-4 text-[#ccd6f6]" />
        </button>

        <div className="px-6 pt-6 pb-2">
          {/* Title */}
          <div className="text-center">
            <h2 className="text-2xl font-bold text-gold-gradient flex items-center justify-center gap-2">
              <RotateCw className="w-6 h-6" />
              Lucky Wheel
            </h2>
            {state && (
              <div className="flex items-center justify-center gap-4 mt-2 text-sm">
                <span className="flex items-center gap-1 text-[#f5a623]">
                  <Gift className="w-3.5 h-3.5" />
                  {state.remaining_free} free
                </span>
                {state.total_spins > 0 && (
                  <span className="flex items-center gap-1 text-[#8892b0]">
                    <History className="w-3.5 h-3.5" />
                    {state.total_spins} total
                  </span>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Wheel area */}
        <div className="relative flex items-center justify-center py-4 px-6">
          {loading ? (
            <div className="w-64 h-64 rounded-full bg-[#1a1a2e] border border-[#f5a623]/20 flex items-center justify-center">
              <Loader2 className="w-8 h-8 text-[#f5a623] animate-spin" />
            </div>
          ) : prizes.length > 0 ? (
            <>
              {/* Pointer arrow */}
              <div className="absolute top-2 z-20">
                <div className="w-0 h-0 border-l-[14px] border-r-[14px] border-t-[28px] border-l-transparent border-r-transparent border-t-[#f5a623] drop-shadow-lg" />
              </div>

              {/* Wheel */}
              <div
                ref={wheelRef}
                className="relative w-64 h-64 md:w-72 md:h-72 rounded-full border-4 border-[#f5a623]/40 shadow-2xl shadow-[#f5a623]/10"
                style={{
                  transform: `rotate(${rotation}deg)`,
                  transition: spinning
                    ? 'transform 4s cubic-bezier(0.17, 0.67, 0.12, 0.99)'
                    : 'none',
                }}
              >
                {prizes.map((prize, idx) => {
                  const startAngle = idx * anglePerPrize;
                  const color = RARITY_COLORS[prize.rarity] || RARITY_COLORS.common;
                  const bgGradient = RARITY_BG[prize.rarity] || RARITY_BG.common;
                  const icon = PRIZE_TYPE_ICONS[prize.type] || '?';

                  return (
                    <div
                      key={prize.id}
                      className="absolute inset-0"
                      style={{
                        clipPath: `polygon(50% 50%, ${50 + 50 * Math.cos(((startAngle - 90) * Math.PI) / 180)}% ${50 + 50 * Math.sin(((startAngle - 90) * Math.PI) / 180)}%, ${50 + 50 * Math.cos(((startAngle + anglePerPrize - 90) * Math.PI) / 180)}% ${50 + 50 * Math.sin(((startAngle + anglePerPrize - 90) * Math.PI) / 180)}%)`,
                        background: `linear-gradient(135deg, ${color}15, ${color}30)`,
                      }}
                    >
                      {/* Prize label */}
                      <div
                        className="absolute flex flex-col items-center justify-center text-center"
                        style={{
                          top: '22%',
                          left: '50%',
                          transform: `rotate(${startAngle + anglePerPrize / 2}deg) translateX(-50%)`,
                          transformOrigin: '0 120px',
                          color: color,
                        }}
                      >
                        <span className="text-lg font-bold leading-none">{icon}</span>
                        <span className="text-[10px] font-medium mt-0.5 leading-tight max-w-[60px] truncate">
                          {prize.name}
                        </span>
                      </div>
                      {/* Separator line */}
                      <div
                        className="absolute top-1/2 left-1/2 w-px h-1/2 origin-top"
                        style={{
                          background: `linear-gradient(to bottom, ${color}60, transparent)`,
                          transform: `rotate(${startAngle}deg)`,
                        }}
                      />
                    </div>
                  );
                })}

                {/* Center circle */}
                <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-16 h-16 rounded-full bg-gradient-to-br from-[#f5a623] to-[#e94560] border-4 border-[#0a0a1a] shadow-lg flex items-center justify-center z-10">
                  <Zap className="w-6 h-6 text-white" />
                </div>
              </div>
            </>
          ) : (
            <div className="w-64 h-64 rounded-full bg-[#1a1a2e] border border-[#f5a623]/20 flex items-center justify-center text-center p-8">
              <p className="text-[#8892b0]">No active wheel activity</p>
            </div>
          )}
        </div>

        {/* Error */}
        {error && (
          <p className="text-center text-sm text-[#e94560] bg-[#e94560]/10 mx-6 px-3 py-2 rounded-lg">{error}</p>
        )}

        {/* Spin buttons */}
        {state && prizes.length > 0 && !showResult && (
          <div className="flex flex-col items-center gap-2 px-6 pb-4">
            <div className="flex gap-3 w-full">
              {state.remaining_free > 0 && (
                <Button
                  onClick={() => handleSpin(true)}
                  disabled={spinning || state.cooldown_remaining > 0}
                  className="flex-1 bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold hover:from-[#ffd700] hover:to-[#f5a623] shadow-lg shadow-[#f5a623]/20 disabled:opacity-50"
                >
                  {spinning ? (
                    <Loader2 className="w-4 h-4 animate-spin mr-2" />
                  ) : (
                    <Gift className="w-4 h-4 mr-2" />
                  )}
                  Free Spin ({state.remaining_free})
                </Button>
              )}
              {state.can_afford_paid && !state.daily_limit_reached && (
                <Button
                  onClick={() => handleSpin(false)}
                  disabled={spinning || state.cooldown_remaining > 0}
                  variant="outline"
                  className="flex-1 border-[#f5a623]/30 text-[#f5a623] hover:bg-[#f5a623]/10 font-semibold disabled:opacity-50"
                >
                  {spinning ? (
                    <Loader2 className="w-4 h-4 animate-spin mr-2" />
                  ) : (
                    <Zap className="w-4 h-4 mr-2" />
                  )}
                  Paid ({state.spin_cost} {state.spin_cost_type})
                </Button>
              )}
            </div>
            {state.cooldown_remaining > 0 && (
              <span className="flex items-center gap-1 text-xs text-[#8892b0]">
                <Clock className="w-3 h-3" />
                Cooldown: {state.cooldown_remaining}s
              </span>
            )}
          </div>
        )}

        {/* Result display */}
        {showResult && spinResult && (
          <div className="px-6 pb-4">
            <div className={`rounded-xl p-4 border ${
              spinResult.prize.type === 'empty'
                ? 'bg-[#1a1a2e] border-[#8892b0]/20'
                : 'bg-gradient-to-br from-[#f5a623]/10 to-[#e94560]/10 border-[#f5a623]/30'
            }`}>
              <p className="text-center text-sm font-medium text-[#8892b0] mb-2">
                {spinResult.prize.type === 'empty' ? 'Better luck next time!' : 'Congratulations!'}
              </p>
              <div className="text-center">
                <span
                  className="text-xl font-bold"
                  style={{ color: RARITY_COLORS[spinResult.prize.rarity] || '#f5a623' }}
                >
                  {spinResult.prize.type === 'empty'
                    ? 'Empty'
                    : spinResult.prize.name}
                </span>
                {spinResult.prize.type !== 'empty' && spinResult.prize.value > 0 && (
                  <p className="text-lg font-semibold text-white mt-1">
                    +{spinResult.prize.value} {spinResult.prize.type === 'bonus' ? 'Bonus' : 'Coin'}
                  </p>
                )}
              </div>
              <Button
                onClick={() => setShowResult(false)}
                className="w-full mt-3 bg-[#1a1a2e] border border-[#f5a623]/20 text-[#ccd6f6] hover:bg-[#1a1a2e]/80"
              >
                Continue
              </Button>
            </div>
          </div>
        )}

        {/* History */}
        {state && state.history.length > 0 && (
          <div className="px-6 pb-4">
            <h3 className="text-xs font-medium text-[#8892b0] mb-2 uppercase tracking-wider">Recent Wins</h3>
            <div className="space-y-1 max-h-32 overflow-y-auto">
              {state.history.map((h, i) => (
                <div key={i} className="flex items-center justify-between text-xs py-1 px-2 rounded bg-[#1a1a2e]/60">
                  <span style={{ color: RARITY_COLORS[h.prize_rarity] || '#8892b0' }}>
                    {h.prize_name}
                  </span>
                  <span className="text-[#8892b0]">
                    {h.prize_type !== 'empty' && h.value > 0 ? `+${h.value}` : ''}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
