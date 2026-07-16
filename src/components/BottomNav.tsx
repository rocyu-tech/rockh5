'use client';

import { usePathname, useRouter } from 'next/navigation';
import { Home, Gamepad2, Gift, User, CheckCircle2 } from 'lucide-react';
import { useAuthStore } from '@/store/auth';

const tabs = [
  { id: '/', label: 'Home', icon: Home },
  { id: '/games', label: 'Games', icon: Gamepad2 },
  { id: '/tasks', label: 'Tasks', icon: CheckCircle2 },
  { id: '/promotions', label: 'Promos', icon: Gift },
  { id: '/profile', label: 'Profile', icon: User, showMailBadge: true },
];

export default function BottomNav() {
  const pathname = usePathname();
  const router = useRouter();
  const isLoggedIn = useAuthStore(s => s.isLoggedIn);
  const unreadMail = useAuthStore(s => s.unreadMailCount);

  const isActive = (tabId: string) => {
    if (tabId === '/') return pathname === '/';
    return pathname.startsWith(tabId);
  };

  return (
    <nav className="fixed bottom-0 left-0 right-0 z-50 bg-[#0a0a1a]/95 backdrop-blur-md border-t border-[#f5a623]/15">
      <div className="flex items-center justify-around h-14 max-w-lg mx-auto">
        {tabs.map((tab) => {
          const active = isActive(tab.id);
          const Icon = tab.icon;
          return (
            <button
              key={tab.id}
              onClick={() => router.push(tab.id)}
              className={`relative flex flex-col items-center justify-center gap-0.5 flex-1 h-full transition-all duration-200 ${
                active
                  ? 'text-[#f5a623]'
                  : 'text-[#8892b0] active:text-[#ccd6f6]'
              }`}
            >
              <div className="relative">
                <Icon className={`w-5 h-5 transition-transform duration-200 ${active ? 'scale-110' : ''}`} />
                {tab.showMailBadge && unreadMail > 0 && (
                  <span className="absolute -top-1 -right-2 min-w-[14px] h-[14px] flex items-center justify-center bg-red-500 text-white text-[8px] font-bold rounded-full px-0.5">
                    {unreadMail > 99 ? '99+' : unreadMail}
                  </span>
                )}
              </div>
              <span className="text-[10px] font-medium">{tab.label}</span>
              {active && (
                <div className="absolute -top-px left-1/2 -translate-x-1/2 w-6 h-0.5 rounded-full bg-[#f5a623]" />
              )}
            </button>
          );
        })}
      </div>
    </nav>
  );
}
