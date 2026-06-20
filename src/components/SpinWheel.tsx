'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { useAuthStore } from '@/store/auth';
import { wheelApi, type WheelConfig, type WheelState, type SpinResult } from '@/lib/api';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog';
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
  common: 'rgba(26, 26, 46, 0.9)',
  rare: 'rgba(13, 59, 58, 0.9)',
  epic: 'rgba(45, 27, 78, 0.9)',
  legendary: 'rgba(61, 43, 14, 0.9)',
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
  const canvasRef = useRef<HTMLCanvasElement>(null);

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

  // Draw wheel on canvas whenever config changes
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const prizes = config?.prizes ?? [];
    if (prizes.length === 0) return;

    const dpr = window.devicePixelRatio || 1;
    const displaySize = 280; // display pixels
    canvas.width = displaySize * dpr;
    canvas.height = displaySize * dpr;
    canvas.style.width = `${displaySize}px`;
    canvas.style.height = `${displaySize}px`;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    ctx.scale(dpr, dpr);
    const cx = displaySize / 2;
    const cy = displaySize / 2;
    const radius = displaySize / 2 - 4;
    const prizeCount = prizes.length;
    const anglePerPrize = (2 * Math.PI) / prizeCount;

    // Draw each sector
    for (let i = 0; i < prizeCount; i++) {
      const startAngle = i * anglePerPrize - Math.PI / 2;
      const endAngle = startAngle + anglePerPrize;
      const midAngle = startAngle + anglePerPrize / 2;
      const prize = prizes[i];
      const color = RARITY_COLORS[prize.rarity] || RARITY_COLORS.common;
      const bgColor = RARITY_BG[prize.rarity] || RARITY_BG.common;

      // Sector fill
      ctx.beginPath();
      ctx.moveTo(cx, cy);
      ctx.arc(cx, cy, radius, startAngle, endAngle);
      ctx.closePath();
      ctx.fillStyle = bgColor;
      ctx.fill();

      // Sector border
      ctx.strokeStyle = color + '40';
      ctx.lineWidth = 1;
      ctx.stroke();

      // Prize icon
      const icon = PRIZE_TYPE_ICONS[prize.type] || '?';
      ctx.save();
      ctx.translate(cx, cy);
      ctx.rotate(midAngle);
      ctx.fillStyle = color;
      ctx.font = 'bold 16px sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(icon, radius * 0.62, 0);

      // Prize name
      ctx.font = '10px sans-serif';
      ctx.fillStyle = color + 'cc';
      ctx.fillText(prize.name, radius * 0.45, 12);
      ctx.restore();
    }

    // Outer ring
    ctx.beginPath();
    ctx.arc(cx, cy, radius, 0, Math.PI * 2);
    ctx.strokeStyle = 'rgba(245, 166, 35, 0.5)';
    ctx.lineWidth = 3;
    ctx.stroke();

    // Decorative dots on outer ring
    for (let i = 0; i < prizeCount; i++) {
      const angle = i * anglePerPrize - Math.PI / 2;
      const dotX = cx + radius * Math.cos(angle);
      const dotY = cy + radius * Math.sin(angle);
      ctx.beginPath();
      ctx.arc(dotX, dotY, 4, 0, Math.PI * 2);
      ctx.fillStyle = '#f5a623';
      ctx.fill();
      ctx.beginPath();
      ctx.arc(dotX, dotY, 2, 0, Math.PI * 2);
      ctx.fillStyle = '#ffd700';
      ctx.fill();
    }

    // Center circle
    const grad = ctx.createRadialGradient(cx, cy, 0, cx, cy, 30);
    grad.addColorStop(0, '#f5a623');
    grad.addColorStop(1, '#e94560');
    ctx.beginPath();
    ctx.arc(cx, cy, 28, 0, Math.PI * 2);
    ctx.fillStyle = grad;
    ctx.fill();
    ctx.strokeStyle = '#0a0a1a';
    ctx.lineWidth = 4;
    ctx.stroke();

    // Center icon
    ctx.fillStyle = '#ffffff';
    ctx.font = 'bold 18px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('\u26A1', cx, cy);
  }, [config]);

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
        const prizes = config?.prizes ?? [];
        const prizeCount = prizes.length;
        const anglePerPrize = 360 / prizeCount;
        // Target: the prize sector should end up at top (0 degrees = 12 o'clock)
        const targetAngle = 360 - (result.prize_index * anglePerPrize + anglePerPrize / 2);
        const fullSpins = 5 + Math.floor(Math.random() * 3);
        const newRotation = rotation + fullSpins * 360 + targetAngle - (rotation % 360);

        setRotation(newRotation);
        setSpinResult(result);

        setTimeout(() => {
          setShowResult(true);
          setSpinning(false);
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

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="bg-[#0a0a1a] border-[#f5a623]/20 text-[#ccd6f6] rounded-2xl max-h-[90vh] overflow-y-auto" showCloseButton={false}>
        <DialogTitle className="sr-only">Lucky Wheel</DialogTitle>
        <DialogDescription className="sr-only">Spin the wheel to win prizes</DialogDescription>

        {/* Close button */}
        <button
          onClick={() => onOpenChange(false)}
          className="absolute top-3 right-3 z-50 w-8 h-8 rounded-full bg-white/10 hover:bg-white/20 flex items-center justify-center transition-colors"
        >
          <X className="w-4 h-4 text-[#ccd6f6]" />
        </button>

        <div className="px-4 pt-5 pb-2">
          {/* Title */}
          <div className="text-center">
            <h2 className="text-xl font-bold text-gold-gradient flex items-center justify-center gap-2">
              <RotateCw className="w-5 h-5" />
              Lucky Wheel
            </h2>
            {state && (
              <div className="flex items-center justify-center gap-4 mt-1.5 text-sm">
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
        <div className="flex flex-col items-center py-3 px-4">
          {loading ? (
            <div className="w-[280px] h-[280px] rounded-full bg-[#1a1a2e] border border-[#f5a623]/20 flex items-center justify-center">
              <Loader2 className="w-8 h-8 text-[#f5a623] animate-spin" />
            </div>
          ) : prizes.length > 0 ? (
            <div className="relative">
              {/* Pointer arrow - fixed at top center */}
              <div className="absolute top-0 left-1/2 -translate-x-1/2 -translate-y-1 z-20">
                <div className="w-0 h-0 border-l-[12px] border-r-[12px] border-t-[24px] border-l-transparent border-r-transparent border-t-[#f5a623] drop-shadow-lg" />
              </div>

              {/* Canvas wheel with CSS rotation */}
              <div
                className="w-[280px] h-[280px]"
                style={{
                  transform: `rotate(${rotation}deg)`,
                  transition: spinning
                    ? 'transform 4s cubic-bezier(0.17, 0.67, 0.12, 0.99)'
                    : 'none',
                }}
              >
                <canvas
                  ref={canvasRef}
                  className="w-full h-full rounded-full"
                />
              </div>
            </div>
          ) : (
            <div className="w-[280px] h-[280px] rounded-full bg-[#1a1a2e] border border-[#f5a623]/20 flex items-center justify-center text-center p-8">
              <p className="text-[#8892b0]">No active wheel activity</p>
            </div>
          )}
        </div>

        {/* Error */}
        {error && (
          <p className="text-center text-sm text-[#e94560] bg-[#e94560]/10 mx-4 px-3 py-2 rounded-lg">{error}</p>
        )}

        {/* Spin buttons */}
        {state && prizes.length > 0 && !showResult && (
          <div className="flex flex-col items-center gap-2 px-4 pb-3">
            <div className="flex gap-3 w-full max-w-[320px]">
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
          <div className="px-4 pb-3">
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
          <div className="px-4 pb-4">
            <h3 className="text-xs font-medium text-[#8892b0] mb-2 uppercase tracking-wider">Recent Wins</h3>
            <div className="space-y-1 max-h-28 overflow-y-auto">
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
