'use client';

import { useState, useEffect, useCallback } from 'react';
import { Gift, CalendarCheck, Clock, Loader2, PartyPopper, CheckCircle2 } from 'lucide-react';
import Navbar from '@/components/Navbar';
import PromotionsSection from '@/components/PromotionsSection';
import { Button } from '@/components/ui/button';
import { useAuthStore } from '@/store/auth';
import { activityApi } from '@/lib/api';
import { toast } from 'sonner';

export default function PromotionsPage() {
  const { isLoggedIn } = useAuthStore();
  const [checkInState, setCheckInState] = useState<{ checked_today: boolean; consecutive_days: number } | null>(null);
  const [timedGift, setTimedGift] = useState<{ available: boolean; next_available_at: string } | null>(null);
  const [checkingIn, setCheckingIn] = useState(false);
  const [claimingGift, setClaimingGift] = useState(false);

  const fetchStates = useCallback(async () => {
    if (!isLoggedIn) return;
    try {
      const [checkInRes, giftRes] = await Promise.all([
        activityApi.getCheckInState().catch(() => null),
        activityApi.getTimedGiftStatus().catch(() => null),
      ]);
      if (checkInRes?.data?.code === 0) setCheckInState(checkInRes.data.data);
      if (giftRes?.data?.code === 0) setTimedGift(giftRes.data.data);
    } catch { /* ignore */ }
  }, [isLoggedIn]);

  useEffect(() => { fetchStates(); }, [fetchStates]);

  const handleCheckIn = async () => {
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
      return;
    }
    setCheckingIn(true);
    try {
      const res = await activityApi.checkIn();
      if (res.data?.code === 0) {
        toast.success(`Checked in! +${res.data.data.bonus_amount} bonus (${res.data.data.consecutive_days} day streak)`);
        await fetchStates();
      } else {
        toast.error(res.data?.message || 'Check-in failed');
      }
    } catch {
      toast.error('Check-in failed');
    } finally {
      setCheckingIn(false);
    }
  };

  const handleClaimTimedGift = async () => {
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
      return;
    }
    setClaimingGift(true);
    try {
      const res = await activityApi.claimTimedGift();
      if (res.data?.code === 0) {
        toast.success(`Received: ${res.data.data.item_name} x${res.data.data.quantity}!`);
        await fetchStates();
      } else {
        toast.error(res.data?.message || 'Claim failed');
      }
    } catch {
      toast.error('Claim failed');
    } finally {
      setClaimingGift(false);
    }
  };

  const handleSpinClick = () => {
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
    } else {
      window.dispatchEvent(new CustomEvent('nav:open-spin'));
    }
  };

  return (
    <div>
      <Navbar
        onLoginClick={() => window.dispatchEvent(new CustomEvent('auth:logout'))}
        onRegisterClick={() => window.dispatchEvent(new CustomEvent('nav:open-register'))}
      />

      <main className="pt-14 px-4">
        {/* Page header */}
        <div className="flex items-center gap-2 mb-4">
          <Gift className="w-5 h-5 text-[#f5a623]" />
          <h1 className="text-lg font-bold text-white">Promotions</h1>
        </div>
        <p className="text-xs text-[#8892b0] mb-4">
          Take advantage of our exclusive bonuses and promotions. Boost your bankroll today!
        </p>

        {/* Daily Check-in Card */}
        {isLoggedIn && checkInState && (
          <div className="bg-[#0d1117] rounded-xl border border-[#1e293b] p-4 mb-4">
            <div className="flex items-center gap-2 mb-3">
              <CalendarCheck className="w-4 h-4 text-[#f5a623]" />
              <h2 className="text-sm font-bold text-white">Daily Check-in</h2>
              {checkInState.consecutive_days > 0 && (
                <span className="text-[10px] bg-[#f5a623]/20 text-[#f5a623] px-2 py-0.5 rounded-full">
                  {checkInState.consecutive_days} day streak
                </span>
              )}
            </div>
            <Button
              onClick={handleCheckIn}
              disabled={checkingIn || checkInState.checked_today}
              className={`w-full h-10 text-sm font-semibold ${
                checkInState.checked_today
                  ? 'bg-[#1e293b] text-[#8892b0] cursor-not-allowed'
                  : 'bg-gradient-to-r from-[#f5a623] to-[#e09100] text-black hover:opacity-90'
              }`}
            >
              {checkingIn ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : checkInState.checked_today ? (
                <>
                  <CheckCircle2 className="w-4 h-4 mr-2" />
                  Checked In Today
                </>
              ) : (
                <>
                  <CalendarCheck className="w-4 h-4 mr-2" />
                  Check In Now
                </>
              )}
            </Button>
          </div>
        )}

        {/* Timed Gift Card */}
        {isLoggedIn && timedGift && (
          <div className="bg-[#0d1117] rounded-xl border border-[#1e293b] p-4 mb-4">
            <div className="flex items-center gap-2 mb-3">
              <Clock className="w-4 h-4 text-[#a855f7]" />
              <h2 className="text-sm font-bold text-white">Timed Gift</h2>
            </div>
            <Button
              onClick={handleClaimTimedGift}
              disabled={claimingGift || !timedGift.available}
              className={`w-full h-10 text-sm font-semibold ${
                !timedGift.available
                  ? 'bg-[#1e293b] text-[#8892b0] cursor-not-allowed'
                  : 'bg-gradient-to-r from-[#a855f7] to-[#7c3aed] text-white hover:opacity-90'
              }`}
            >
              {claimingGift ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : !timedGift.available ? (
                <>
                  <Clock className="w-4 h-4 mr-2" />
                  Available at {new Date(timedGift.next_available_at).toLocaleTimeString()}
                </>
              ) : (
                <>
                  <PartyPopper className="w-4 h-4 mr-2" />
                  Claim Gift
                </>
              )}
            </Button>
          </div>
        )}

        {/* Promotions list */}
        <PromotionsSection onSpinClick={handleSpinClick} />
      </main>
    </div>
  );
}
