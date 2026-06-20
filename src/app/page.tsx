'use client';

import Navbar from '@/components/Navbar';
import BannerCarousel from '@/components/BannerCarousel';
import { useAuthStore } from '@/store/auth';
import { Gamepad2, Users, Trophy, Shield, Wallet, Gift, Crown, RotateCw } from 'lucide-react';

export default function Home() {
  const { isLoggedIn } = useAuthStore();

  const handleSpinClick = () => {
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
    } else {
      window.dispatchEvent(new CustomEvent('nav:open-spin'));
    }
  };

  const handleQuickAction = (path: string) => {
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
      return;
    }
    window.location.href = path;
  };

  return (
    <div>
      <Navbar
        onLoginClick={() => window.dispatchEvent(new CustomEvent('auth:logout'))}
        onRegisterClick={() => {
          // Trigger register modal - dispatch custom event
          window.dispatchEvent(new CustomEvent('nav:open-register'));
        }}
      />

      <main className="pt-14 px-4">
        {/* Banner */}
        <BannerCarousel />

        {/* Stats bar - 2x2 grid */}
        <div className="grid grid-cols-2 gap-2.5 mt-4">
          {[
            { icon: Gamepad2, label: 'Games', value: '5,000+', color: '#f5a623' },
            { icon: Users, label: 'Players', value: '100K+', color: '#e94560' },
            { icon: Trophy, label: 'Jackpots', value: '$10M+', color: '#4ecdc4' },
            { icon: Shield, label: 'Licensed', value: '100%', color: '#a855f7' },
          ].map((stat) => (
            <div
              key={stat.label}
              className="flex items-center gap-2.5 p-3 rounded-xl bg-[#1a1a2e]/60 border border-[#f5a623]/10 backdrop-blur-sm"
            >
              <div
                className="w-9 h-9 rounded-lg flex items-center justify-center flex-shrink-0"
                style={{ backgroundColor: `${stat.color}15` }}
              >
                <stat.icon className="w-4 h-4" style={{ color: stat.color }} />
              </div>
              <div>
                <p className="text-[10px] text-[#8892b0]">{stat.label}</p>
                <p className="text-sm font-bold text-white">{stat.value}</p>
              </div>
            </div>
          ))}
        </div>

        {/* Quick Actions */}
        <div className="mt-6">
          <h2 className="text-base font-bold text-white mb-3">Quick Access</h2>
          <div className="grid grid-cols-4 gap-3">
            {[
              { icon: Wallet, label: 'Deposit', color: '#4ecdc4', bg: '#4ecdc4', path: '/profile' },
              { icon: Gift, label: 'Promos', color: '#a855f7', bg: '#a855f7', path: '/promotions' },
              { icon: Crown, label: 'VIP', color: '#f5a623', bg: '#f5a623', path: '/profile' },
              { icon: RotateCw, label: 'Lucky', color: '#e94560', bg: '#e94560', action: 'spin' },
            ].map((item) => (
              <button
                key={item.label}
                onClick={() => item.action === 'spin' ? handleSpinClick() : handleQuickAction(item.path)}
                className="flex flex-col items-center gap-1.5 active:scale-95 transition-transform"
              >
                <div
                  className="w-12 h-12 rounded-2xl flex items-center justify-center"
                  style={{
                    background: `linear-gradient(135deg, ${item.bg}20, ${item.bg}08)`,
                    border: `1px solid ${item.bg}30`,
                  }}
                >
                  <item.icon className="w-5 h-5" style={{ color: item.color }} />
                </div>
                <span className="text-[11px] font-medium text-[#ccd6f6]">{item.label}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Hot Games Preview */}
        <div className="mt-6">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Gamepad2 className="w-4 h-4 text-[#f5a623]" />
              Hot Games
            </h2>
            <button
              onClick={() => window.location.href = '/games'}
              className="text-xs text-[#f5a623] font-medium active:opacity-70"
            >
              View All →
            </button>
          </div>
          <div className="grid grid-cols-3 gap-2.5">
            {[
              { name: 'Fortune Tiger', vendor: 'PG Soft', gradient: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)' },
              { name: 'Sweet Bonanza', vendor: 'Pragmatic', gradient: 'linear-gradient(135deg, #f093fb 0%, #f5576c 100%)' },
              { name: 'Aviator', vendor: 'Spribe', gradient: 'linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)' },
            ].map((game) => (
              <div
                key={game.name}
                className="relative rounded-xl overflow-hidden aspect-[4/5] cursor-pointer active:scale-[0.98] transition-transform"
                style={{ background: game.gradient }}
              >
                <div className="absolute inset-0 flex items-center justify-center opacity-20">
                  <Gamepad2 className="w-12 h-12 text-white" />
                </div>
                <div className="absolute bottom-0 left-0 right-0 p-2 bg-gradient-to-t from-black/80 to-transparent">
                  <p className="text-xs font-semibold text-white truncate">{game.name}</p>
                  <p className="text-[10px] text-white/60">{game.vendor}</p>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Latest Promotions Preview */}
        <div className="mt-6 mb-4">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-base font-bold text-white flex items-center gap-2">
              <Gift className="w-4 h-4 text-[#f5a623]" />
              Promotions
            </h2>
            <button
              onClick={() => window.location.href = '/promotions'}
              className="text-xs text-[#f5a623] font-medium active:opacity-70"
            >
              View All →
            </button>
          </div>
          <div className="space-y-2.5">
            {[
              { title: 'Welcome Bonus 200%', desc: 'Get a 200% welcome bonus on your first deposit up to $1,000!', gradient: 'linear-gradient(135deg, #1a1a2e 0%, #0f3460 100%)', badge: 'HOT' },
              { title: 'Weekly Cashback 15%', desc: 'Enjoy 15% cashback on your weekly losses.', gradient: 'linear-gradient(135deg, #1a1a2e 0%, #533483 100%)', badge: 'POPULAR' },
            ].map((promo) => (
              <div
                key={promo.title}
                className="rounded-xl overflow-hidden border border-[#f5a623]/10"
              >
                <div className="relative p-4" style={{ background: promo.gradient }}>
                  <div className="flex items-center gap-2 mb-2">
                    <span className="px-2 py-0.5 rounded-full bg-[#e94560]/20 text-[#e94560] text-[10px] font-semibold">
                      {promo.badge}
                    </span>
                  </div>
                  <h3 className="text-sm font-bold text-white mb-1">{promo.title}</h3>
                  <p className="text-xs text-[#8892b0] line-clamp-1">{promo.desc}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </main>
    </div>
  );
}
