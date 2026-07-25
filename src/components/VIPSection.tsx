'use client';

import { useState, useEffect } from 'react';
import { Progress } from '@/components/ui/progress';
import { Crown, Shield, Star, Diamond, Gem, Zap, Trophy, Award, ChevronRight } from 'lucide-react';
import { useAuthStore } from '@/store/auth';
import { vipApi } from '@/lib/api';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';
import DemoBadge from '@/components/DemoBadge';
import type { VIPLevel } from '@/lib/api';

const defaultVIPLevels: VIPLevel[] = [
  {
    level: 1,
    name: 'Bronze',
    growth_required: 0,
    benefits: ['5% Cashback', 'Birthday Bonus', 'Priority Support'],
    icon: '🥉',
  },
  {
    level: 2,
    name: 'Silver',
    growth_required: 1000,
    benefits: ['8% Cashback', 'Weekly Bonus', 'Personal Manager', 'Exclusive Games'],
    icon: '🥈',
  },
  {
    level: 3,
    name: 'Gold',
    growth_required: 5000,
    benefits: ['10% Cashback', 'Daily Bonus', 'Personal Manager', 'VIP Events', 'Higher Limits'],
    icon: '🥇',
  },
  {
    level: 4,
    name: 'Platinum',
    growth_required: 20000,
    benefits: ['12% Cashback', 'Unlimited Bonuses', 'Dedicated Manager', 'VIP Events', 'Luxury Gifts', 'Priority Withdrawal'],
    icon: '💎',
  },
  {
    level: 5,
    name: 'Diamond',
    growth_required: 100000,
    benefits: ['15% Cashback', 'All Bonuses', '24/7 Manager', 'Exclusive Events', 'Luxury Gifts', 'Instant Withdrawal', 'Custom Limits'],
    icon: '👑',
  },
];

const levelColors: Record<string, string> = {
  Bronze: '#cd7f32',
  Silver: '#c0c0c0',
  Gold: '#f5a623',
  Platinum: '#4ecdc4',
  Diamond: '#a855f7',
};

const levelIcons: Record<string, React.ReactNode> = {
  Bronze: <Shield className="w-6 h-6" />,
  Silver: <Star className="w-6 h-6" />,
  Gold: <Crown className="w-6 h-6" />,
  Platinum: <Diamond className="w-6 h-6" />,
  Diamond: <Trophy className="w-6 h-6" />,
};

export default function VIPSection() {
  const { isLoggedIn, user } = useAuthStore();
  const [levels, setLevels] = useState<VIPLevel[]>(defaultVIPLevels);
  const [vipInfo, setVipInfo] = useState<{ level: number; progress: number } | null>(null);
  const [usingDemo, setUsingDemo] = useState(false);
  const apiStatus = useApiStatusContext();

  useEffect(() => {
    vipApi.getLevels().then((res) => {
      const list = res.data?.levels;
      if (list?.length) setLevels(list);
    }).catch((err) => {
      setUsingDemo(true);
      apiStatus.markFailed('vip/levels', getErrorMessage(err));
    });
    if (isLoggedIn) {
      vipApi.getInfo().then((res) => {
        if (res.data) setVipInfo(res.data);
      }).catch((err) => {
        apiStatus.markFailed('vip/info', getErrorMessage(err));
      });
    }
  }, [isLoggedIn]);

  const currentLevel = vipInfo?.level ?? user?.vip_level ?? 1;
  const currentLevelData = levels.find((l) => l.level === currentLevel) ?? levels[0];
  const nextLevelData = levels.find((l) => l.level === currentLevel + 1);
  const progress = vipInfo?.progress ?? (user ? 45 : 0);

  return (
    <div className="space-y-6">
      {/* User VIP Status */}
      {isLoggedIn && (
        <div className="rounded-xl bg-gradient-to-r from-[#1a1a2e] to-[#16213e] border border-[#f5a623]/20 p-4 md:p-6">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-3">
              <span className="text-2xl">{currentLevelData.icon}</span>
              <div>
                <p className="text-sm text-[#8892b0]">Current VIP Level</p>
                <p className="text-lg font-bold" style={{ color: levelColors[currentLevelData.name] || '#f5a623' }}>
                  {currentLevelData.name}
                </p>
              </div>
            </div>
            {nextLevelData && (
              <div className="text-right">
                <p className="text-sm text-[#8892b0]">Next Level</p>
                <p className="text-sm font-medium" style={{ color: levelColors[nextLevelData.name] || '#f5a623' }}>
                  {nextLevelData.icon} {nextLevelData.name}
                </p>
              </div>
            )}
          </div>
          <div className="space-y-2">
            <div className="flex justify-between text-xs text-[#8892b0]">
              <span>{currentLevelData.name}</span>
              <span>{progress}%</span>
              {nextLevelData && <span>{nextLevelData.name}</span>}
            </div>
            <Progress value={progress} className="h-2 bg-[#0a0a1a]" />
          </div>
        </div>
      )}

      {/* VIP Levels Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-3 md:gap-4">
        {levels.map((level) => {
          const color = levelColors[level.name] || '#f5a623';
          const isCurrentLevel = level.level === currentLevel;
          const isLocked = level.level > currentLevel + 1;

          return (
            <div
              key={level.level}
              className={`relative rounded-xl border p-4 transition-all duration-300 ${
                isCurrentLevel
                  ? 'border-[#f5a623]/50 bg-gradient-to-b from-[#f5a623]/10 to-[#1a1a2e] shadow-lg'
                  : isLocked
                    ? 'border-white/5 bg-[#1a1a2e]/50 opacity-60'
                    : 'border-white/10 bg-[#1a1a2e] hover:border-white/20'
              }`}
            >
              {/* Level badge */}
              <div className="flex items-center gap-2 mb-3">
                <span className="text-xl">{level.icon}</span>
                <div>
                  <p className="text-sm font-bold" style={{ color }}>
                    {level.name}
                  </p>
                  <p className="text-[10px] text-[#8892b0]">Level {level.level}</p>
                </div>
                {isCurrentLevel && (
                  <span className="ml-auto px-2 py-0.5 rounded-full text-[10px] font-semibold bg-[#f5a623]/20 text-[#f5a623]">
                    CURRENT
                  </span>
                )}
              </div>

              {/* Growth required */}
              <div className="mb-3">
                <p className="text-[10px] text-[#8892b0] uppercase tracking-wider">Growth Required</p>
                <p className="text-sm font-semibold text-white">
                  {level.growth_required.toLocaleString()} pts
                </p>
              </div>

              {/* Benefits */}
              <div className="space-y-1.5">
                {level.benefits.slice(0, 3).map((benefit, idx) => (
                  <div key={idx} className="flex items-center gap-1.5 text-xs text-[#ccd6f6]">
                    <ChevronRight className="w-3 h-3 flex-shrink-0" style={{ color }} />
                    <span className="truncate">{benefit}</span>
                  </div>
                ))}
                {level.benefits.length > 3 && (
                  <p className="text-[10px] text-[#8892b0]">+{level.benefits.length - 3} more</p>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
