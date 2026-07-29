'use client';

// VIP Club page — shows the user's current VIP level, progress to next tier,
// all VIP levels with their perks, and a daily check-in entry point.
//
// Endpoints used:
//   GET  /api/v1/vip/levels?lang=zh|en — list all configured levels
//   GET  /api/v1/vip/info              — current user's level + growth + progress
//   GET  /api/v1/activity/check-in/state — to know if today's check-in is done
//   POST /api/v1/activity/check-in    — perform check-in (also credits VIP bonus)

import { useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Crown, Sparkles, CheckCircle2, Loader2, Coins, TrendingUp, Gift, ChevronRight } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useLocale } from '@/i18n/provider';
import { vipApi, activityApi } from '@/lib/api';
import { getErrorMessage } from '@/lib/api-status';
import { useAuthStore } from '@/store/auth';
import { toast } from 'sonner';
import Navbar from '@/components/Navbar';
import { Progress } from '@/components/ui/progress';
import { fmtMoney, fmtMoneyPlain } from '@/lib/money';

interface VIPLevel {
  level: number;
  name: string;
  growth_required: number;
  benefits: string[];
  icon?: string;
  withdraw_fee_rate?: number;
  daily_signin_bonus?: number;
}

interface VIPInfo {
  level: number;
  growth: number;
  progress: number;
  next_level?: { level: number; name: string; growth_required: number } | Record<string, never>;
}

// Fallback demo data — used only if the backend hasn't seeded vip_level_config.
// Helps the UI render something useful in fresh dev environments.
const DEMO_LEVELS: VIPLevel[] = [
  { level: 1, name: 'Bronze', growth_required: 0, benefits: ['Daily Withdraw Limit', 'Birthday Bonus'], icon: '🥉', withdraw_fee_rate: 5, daily_signin_bonus: 100 },
  { level: 2, name: 'Silver', growth_required: 1000, benefits: ['Daily Withdraw Limit', 'Birthday Bonus', 'Exclusive Activity'], icon: '🥈', withdraw_fee_rate: 4, daily_signin_bonus: 200 },
  { level: 3, name: 'Gold', growth_required: 5000, benefits: ['Higher Withdraw Limit', 'Birthday Bonus', 'Exclusive Activity', 'Exclusive Support'], icon: '🥇', withdraw_fee_rate: 3, daily_signin_bonus: 500 },
  { level: 4, name: 'Platinum', growth_required: 20000, benefits: ['Premium Withdraw Limit', 'Birthday Bonus', 'Exclusive Activity', 'Exclusive Support', 'All Benefits'], icon: '💎', withdraw_fee_rate: 2, daily_signin_bonus: 1000 },
  { level: 5, name: 'Diamond', growth_required: 100000, benefits: ['VIP Withdraw Limit', 'Birthday Bonus', 'Exclusive Activity', 'Exclusive Support', 'All Benefits'], icon: '👑', withdraw_fee_rate: 0, daily_signin_bonus: 5000 },
];

const levelColors: Record<string, string> = {
  Bronze: '#cd7f32',
  Silver: '#c0c0c0',
  Gold: '#ffd700',
  Platinum: '#e5e4e2',
  Diamond: '#b9f2ff',
};

export default function VIPPage() {
  const router = useRouter();
  const t = useTranslations();
  const { locale } = useLocale();
  const { isLoggedIn, user } = useAuthStore();

  const [levels, setLevels] = useState<VIPLevel[]>(DEMO_LEVELS);
  const [info, setInfo] = useState<VIPInfo | null>(null);
  const [checkedIn, setCheckedIn] = useState(false);
  const [loading, setLoading] = useState(true);
  const [checkingIn, setCheckingIn] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      try {
        // Fetch levels in the current UI language; backend supports en/zh/vi/th.
        const levelsRes = await vipApi.getLevels(locale);
        const levelsList = levelsRes.data?.levels;
        if (!cancelled && levelsList?.length) {
          setLevels(levelsList);
        }
        if (isLoggedIn) {
          const [infoRes, stateRes] = await Promise.all([
            vipApi.getInfo(),
            activityApi.getCheckInState(),
          ]);
          if (!cancelled) {
            if (infoRes.data) setInfo(infoRes.data);
            // Backend returns `checked_today`; older docs mention `checked_in` — accept both.
            const sd = stateRes.data as { checked_today?: boolean; checked_in?: boolean } | undefined;
            if (sd) setCheckedIn(!!(sd.checked_today ?? sd.checked_in));
          }
        }
      } catch (err) {
        console.error('[vip] load failed:', err);
        toast.error(getErrorMessage(err));
        // Keep DEMO_LEVELS as fallback
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => { cancelled = true; };
  }, [isLoggedIn, locale]);

  const handleCheckIn = useCallback(async () => {
    if (checkingIn || checkedIn) return;
    setCheckingIn(true);
    try {
      await activityApi.checkIn();
      setCheckedIn(true);
      toast.success(locale === 'zh' ? '签到成功！' : 'Checked in!');
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '';
      if (msg.includes('already') || msg.includes('50003')) {
        setCheckedIn(true);
        toast.info(locale === 'zh' ? '今日已签到' : 'Already checked in today');
      } else {
        toast.error(msg || (locale === 'zh' ? '签到失败' : 'Check-in failed'));
      }
    } finally {
      setCheckingIn(false);
    }
  }, [checkingIn, checkedIn, locale]);

  const currentLevel = info?.level ?? user?.vip_level ?? 0;
  const currentLevelData = levels.find((l) => l.level === currentLevel);
  const nextLevelData = info?.next_level && 'level' in info.next_level && info.next_level.level
    ? info.next_level
    : levels.find((l) => l.level === currentLevel + 1);
  const progress = info?.progress ?? 0;
  const dailyBonus = currentLevelData?.daily_signin_bonus ?? 0;

  return (
    <div className="min-h-screen bg-[#0a0a1a] text-white">
      <Navbar onLoginClick={() => router.push('/')} onRegisterClick={() => router.push('/')} />

      <main className="pt-14 px-4 pb-20 max-w-2xl mx-auto">
        {/* Header */}
        <div className="flex items-center gap-3 mb-6 mt-4">
          <Crown className="w-6 h-6 text-[#f5a623]" />
          <h1 className="text-xl font-bold">{t('vip.title')}</h1>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-6 h-6 animate-spin text-[#f5a623]" />
          </div>
        ) : (
          <>
            {/* Current Level Card */}
            {isLoggedIn && (
              <div className="rounded-2xl bg-gradient-to-br from-[#1a1a2e] via-[#16213e] to-[#1a1a2e] border border-[#f5a623]/30 p-6 mb-6 shadow-xl">
                <div className="flex items-start justify-between mb-4">
                  <div>
                    <p className="text-xs text-[#8892b0] uppercase tracking-wider mb-1">{t('vip.currentLevel')}</p>
                    <div className="flex items-center gap-2">
                      <span className="text-3xl">{currentLevelData?.icon || '⭐'}</span>
                      <div>
                        <p className="text-2xl font-bold" style={{ color: levelColors[currentLevelData?.name || ''] || '#f5a623' }}>
                          {currentLevelData?.name || (locale === 'zh' ? '未定级' : 'Unranked')}
                        </p>
                        <p className="text-xs text-[#8892b0]">{t('vip.level')} {currentLevel}</p>
                      </div>
                    </div>
                  </div>
                  {nextLevelData && (
                    <div className="text-right">
                      <p className="text-xs text-[#8892b0] uppercase tracking-wider mb-1">{t('vip.nextLevel')}</p>
                      <p className="text-sm font-semibold text-[#f5a623]">
                        {nextLevelData.name}
                      </p>
                      <p className="text-[10px] text-[#8892b0]">
                        {t('vip.growthRequired')}: {nextLevelData.growth_required.toLocaleString()}
                      </p>
                    </div>
                  )}
                </div>

                {/* Growth + Progress */}
                <div className="space-y-3">
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-[#8892b0] flex items-center gap-1">
                      <TrendingUp className="w-4 h-4" /> {t('vip.growth')}
                    </span>
                    <span className="font-bold">{(info?.growth ?? 0).toLocaleString()}</span>
                  </div>
                  {nextLevelData && (
                    <div>
                      <div className="flex justify-between text-xs text-[#8892b0] mb-1">
                        <span>{t('vip.progress')}</span>
                        <span>{progress.toFixed(1)}%</span>
                      </div>
                      <Progress value={progress} className="h-2 bg-[#0a0a1a]" />
                    </div>
                  )}
                  {!nextLevelData && (
                    <div className="flex items-center gap-2 text-[#f5a623] text-sm font-semibold">
                      <Sparkles className="w-4 h-4" /> {t('vip.maxLevel')}
                    </div>
                  )}
                </div>

                {/* Daily check-in */}
                <div className="mt-5 pt-4 border-t border-white/5">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-xs text-[#8892b0] uppercase tracking-wider mb-1">{t('vip.dailySigninBonus')}</p>
                      <p className="flex items-center gap-1 text-lg font-bold text-[#f5a623]">
                        <Coins className="w-4 h-4" />
                        {fmtMoneyPlain(dailyBonus)}
                      </p>
                    </div>
                    <button
                      onClick={handleCheckIn}
                      disabled={checkingIn || checkedIn}
                      className="px-4 py-2 rounded-lg font-bold text-sm transition-all disabled:cursor-not-allowed flex items-center gap-2"
                      style={{
                        background: checkedIn ? 'rgba(255,255,255,0.05)' : '#f5a623',
                        color: checkedIn ? '#8892b0' : '#000',
                      }}
                    >
                      {checkingIn ? <Loader2 className="w-4 h-4 animate-spin" /> : checkedIn ? <CheckCircle2 className="w-4 h-4" /> : <Gift className="w-4 h-4" />}
                      {checkedIn ? t('vip.checkedIn') : t('vip.checkinNow')}
                    </button>
                  </div>
                </div>
              </div>
            )}

            {!isLoggedIn && (
              <div className="rounded-xl border border-[#f5a623]/20 bg-[#1a1a2e] p-6 mb-6 text-center">
                <p className="text-sm text-[#8892b0]">
                  {locale === 'zh' ? '登录后查看你的 VIP 等级与特权' : 'Log in to see your VIP level and benefits'}
                </p>
                <button
                  onClick={() => router.push('/')}
                  className="mt-3 px-4 py-2 rounded-lg bg-[#f5a623] text-black text-sm font-bold hover:opacity-90"
                >
                  {t('common.login')}
                </button>
              </div>
            )}

            {/* All VIP Levels */}
            <h2 className="text-lg font-bold mb-4 flex items-center gap-2">
              <Crown className="w-5 h-5 text-[#f5a623]" />
              {t('vip.levelsTitle')}
            </h2>
            <div className="space-y-3">
              {levels.map((level) => {
                const color = levelColors[level.name] || '#f5a623';
                const isCurrent = level.level === currentLevel;
                const isLocked = level.level > currentLevel + 1;
                return (
                  <div
                    key={level.level}
                    className={`rounded-xl border p-4 transition-all ${
                      isCurrent
                        ? 'border-[#f5a623]/50 bg-gradient-to-r from-[#f5a623]/10 to-[#1a1a2e]'
                        : isLocked
                          ? 'border-white/5 bg-[#1a1a2e]/50 opacity-60'
                          : 'border-white/10 bg-[#1a1a2e] hover:border-white/20'
                    }`}
                  >
                    <div className="flex items-start justify-between mb-2">
                      <div className="flex items-center gap-3">
                        <span className="text-2xl">{level.icon || '⭐'}</span>
                        <div>
                          <p className="font-bold text-base" style={{ color }}>{level.name}</p>
                          <p className="text-xs text-[#8892b0]">{t('vip.level')} {level.level}</p>
                        </div>
                      </div>
                      {isCurrent && (
                        <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-[#f5a623]/20 text-[#f5a623]">
                          {locale === 'zh' ? '当前' : 'CURRENT'}
                        </span>
                      )}
                    </div>
                    <div className="grid grid-cols-2 gap-2 text-xs mb-3">
                      <div>
                        <p className="text-[#8892b0]">{t('vip.growthRequired')}</p>
                        <p className="font-semibold">{level.growth_required.toLocaleString()}</p>
                      </div>
                      <div>
                        <p className="text-[#8892b0]">{t('vip.withdrawFeeRate')}</p>
                        <p className="font-semibold">{level.withdraw_fee_rate ?? 0}%</p>
                      </div>
                    </div>
                    {level.benefits && level.benefits.length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        {level.benefits.map((b, i) => (
                          <span key={i} className="text-[10px] px-2 py-0.5 rounded bg-white/5 text-[#8892b0]">
                            {b}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>

            {/* Footer link to history */}
            <div className="mt-8 pt-4 border-t border-white/5">
              <button
                onClick={() => router.push('/history')}
                className="w-full flex items-center justify-between p-3 rounded-lg bg-[#1a1a2e] hover:bg-[#16213e] transition-colors"
              >
                <span className="text-sm text-[#8892b0]">{t('nav.history')}</span>
                <ChevronRight className="w-4 h-4 text-[#8892b0]" />
              </button>
            </div>
          </>
        )}
      </main>
    </div>
  );
}
