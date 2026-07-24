'use client';

import { useState, useEffect } from 'react';
import { activityApi, type Activity } from '@/lib/api';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';
import DemoBadge from '@/components/DemoBadge';
import { Gift, ExternalLink, Clock, Flame, RotateCw } from 'lucide-react';

interface PromotionsSectionProps {
  onSpinClick?: () => void;
}

const defaultActivities: Activity[] = [
  {
    id: 1,
    title: 'Welcome Bonus 200%',
    description: 'Get a 200% welcome bonus on your first deposit up to $1,000!',
    image_url: '',
    start_time: '2024-01-01',
    end_time: '2025-12-31',
    status: 1,
  },
  {
    id: 2,
    title: 'Weekly Cashback 15%',
    description: 'Enjoy 15% cashback on your weekly losses. No wagering required!',
    image_url: '',
    start_time: '2024-01-01',
    end_time: '2025-12-31',
    status: 1,
  },
  {
    id: 3,
    title: 'Slots Tournament $50K',
    description: 'Compete in our monthly slots tournament with a $50,000 prize pool!',
    image_url: '',
    start_time: '2024-06-01',
    end_time: '2025-06-30',
    status: 1,
  },
  {
    id: 4,
    title: 'Refer & Earn $100',
    description: 'Invite friends and earn $100 for each referral who deposits!',
    image_url: '',
    start_time: '2024-01-01',
    end_time: '2025-12-31',
    status: 1,
  },
  {
    id: 5,
    title: 'Daily Reload 50%',
    description: 'Get 50% bonus on every daily deposit. Boost your play every day!',
    image_url: '',
    start_time: '2024-01-01',
    end_time: '2025-12-31',
    status: 1,
  },
  {
    id: 6,
    title: 'VIP Exclusive Rewards',
    description: 'Unlock exclusive bonuses, gifts, and experiences as a VIP member!',
    image_url: '',
    start_time: '2024-01-01',
    end_time: '2025-12-31',
    status: 1,
  },
];

const promoGradients = [
  'linear-gradient(135deg, #1a1a2e 0%, #0f3460 100%)',
  'linear-gradient(135deg, #1a1a2e 0%, #533483 100%)',
  'linear-gradient(135deg, #1a1a2e 0%, #e94560 100%)',
  'linear-gradient(135deg, #1a1a2e 0%, #0f3460 50%, #533483 100%)',
  'linear-gradient(135deg, #2e1a1a 0%, #e94560 100%)',
  'linear-gradient(135deg, #1a2e1a 0%, #4ecdc4 100%)',
];

const promoIcons = ['\u{1F381}', '\u{1F525}', '\u{1F3C6}', '\u{1F4B0}', '\u26A1', '\u{1F451}'];

export default function PromotionsSection({ onSpinClick }: PromotionsSectionProps) {
  const [activities, setActivities] = useState<Activity[]>(defaultActivities);
  const [usingDemo, setUsingDemo] = useState(false);
  const apiStatus = useApiStatusContext();

  useEffect(() => {
    activityApi.getList().then((res) => {
      if (res.data?.length) {
        setActivities(res.data);
      }
    }).catch((err) => {
      setUsingDemo(true);
      apiStatus.markFailed('activity/list', getErrorMessage(err));
    });
  }, []);

  return (
    <div className="space-y-4">
      {usingDemo && <DemoBadge show label="Demo Promotions" />}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 md:gap-6">
      {/* Lucky Wheel entry card */}
      {onSpinClick && (
        <div
          onClick={onSpinClick}
          className="group relative rounded-xl overflow-hidden border-2 border-[#f5a623]/30 hover:border-[#f5a623]/60 transition-all duration-300 hover:shadow-xl hover:shadow-[#f5a623]/15 cursor-pointer"
        >
          <div className="relative p-5 md:p-6 min-h-[200px] bg-gradient-to-br from-[#2e1a0e] via-[#1a1a2e] to-[#0e2e2e]">
            {/* Animated wheel icon */}
            <div className="absolute top-4 right-4 w-16 h-16 rounded-full border-2 border-[#f5a623]/40 flex items-center justify-center group-hover:animate-spin" style={{ animationDuration: '3s' }}>
              <RotateCw className="w-8 h-8 text-[#f5a623]/60" />
            </div>
            {/* Badge */}
            <div className="flex items-center gap-2 mb-3">
              <span className="flex items-center gap-1 px-2 py-0.5 rounded-full bg-[#f5a623]/20 text-[#f5a623] text-[10px] font-semibold">
                <Gift className="w-3 h-3" />
                FEATURED
              </span>
              <span className="flex items-center gap-1 px-2 py-0.5 rounded-full bg-[#4ecdc4]/20 text-[#4ecdc4] text-[10px] font-semibold">
                FREE DAILY
              </span>
            </div>
            <h3 className="text-lg md:text-xl font-bold text-gold-gradient mb-2 pr-16">
              Lucky Wheel
            </h3>
            <p className="text-sm text-[#8892b0] mb-4 line-clamp-2">
              Spin the wheel daily for a chance to win bonus, coins and exclusive prizes!
            </p>
            <div className="inline-flex items-center gap-2 px-4 py-3 rounded-lg bg-gradient-to-r from-[#f5a623] to-[#e94560] text-white text-sm font-semibold shadow-lg shadow-[#f5a623]/30 group-hover:shadow-[#f5a623]/50 transition-all">
              <RotateCw className="w-4 h-4 group-hover:animate-spin" style={{ animationDuration: '1s' }} />
              Spin Now
            </div>
          </div>
        </div>
      )}

      {activities.map((activity, index) => (
        <div
          key={activity.id}
          className="group relative rounded-xl overflow-hidden border border-[#f5a623]/10 hover:border-[#f5a623]/30 transition-all duration-300 hover:shadow-lg hover:shadow-[#f5a623]/5"
        >
          {/* Card background */}
          <div
            className="relative p-5 md:p-6 min-h-[200px]"
            style={{ background: promoGradients[index % promoGradients.length] }}
          >
            {/* Decorative pattern */}
            <div className="absolute top-3 right-3 text-4xl opacity-20">
              {promoIcons[index % promoIcons.length]}
            </div>

            {/* Badge */}
            <div className="flex items-center gap-2 mb-3">
              <span className="flex items-center gap-1 px-2 py-0.5 rounded-full bg-[#e94560]/20 text-[#e94560] text-[10px] font-semibold">
                <Flame className="w-3 h-3" />
                {index === 0 ? 'HOT' : index < 3 ? 'POPULAR' : 'LIMITED'}
              </span>
              {activity.start_time && (
                <span className="flex items-center gap-1 text-[10px] text-[#8892b0]">
                  <Clock className="w-3 h-3" />
                  Ongoing
                </span>
              )}
            </div>

            {/* Title */}
            <h3 className="text-lg md:text-xl font-bold text-white mb-2 pr-12">
              {activity.title}
            </h3>

            {/* Description */}
            <p className="text-sm text-[#8892b0] mb-4 line-clamp-2">
              {activity.description}
            </p>

            {/* CTA */}
            <button className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] text-sm font-semibold hover:from-[#ffd700] hover:to-[#f5a623] transition-all shadow-lg shadow-[#f5a623]/20 group-hover:shadow-[#f5a623]/30">
              Join Now
              <ExternalLink className="w-3.5 h-3.5 group-hover:translate-x-0.5 transition-transform" />
            </button>
          </div>
        </div>
      ))}
      </div>
    </div>
  );
}
