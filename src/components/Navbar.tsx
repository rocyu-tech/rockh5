'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/button';
import { Wallet, LogIn, UserPlus, User, LogOut, Gamepad2, Bell } from 'lucide-react';
import { fmtMoney, fmtMoneyPlain } from '@/lib/money';

interface NavbarProps {
  onLoginClick: () => void;
  onRegisterClick: () => void;
}

export default function Navbar({ onLoginClick, onRegisterClick }: NavbarProps) {
  const router = useRouter();
  const { isLoggedIn, user, assets, logout } = useAuthStore();
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const handleScroll = () => setScrolled(window.scrollY > 20);
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  return (
    <header
      className={`fixed top-0 left-0 right-0 z-50 transition-all duration-300 ${
        scrolled
          ? 'bg-[#0a0a1a]/95 backdrop-blur-md border-b border-[#f5a623]/20 shadow-lg shadow-black/20'
          : 'bg-transparent'
      }`}
    >
      <div className="flex items-center justify-between h-12 px-4">
        {/* Logo */}
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center">
            <Gamepad2 className="w-4 h-4 text-white" />
          </div>
          <span className="text-base font-bold text-gold-gradient tracking-tight">
            RockGame
          </span>
        </div>

        {/* Right side */}
        <div className="flex items-center gap-2">
          {isLoggedIn ? (
            <>
              {/* Balance */}
              <button
                onClick={() => router.push('/wallet')}
                aria-label="View wallet"
                className="flex items-center gap-1.5 px-2.5 py-1 min-h-[36px] rounded-lg bg-[#16213e] border border-[#f5a623]/20 active:bg-[#16213e]/80"
              >
                <Wallet className="w-3.5 h-3.5 text-[#f5a623]" />
                <span className="text-xs font-semibold text-[#f5a623]">
                  {fmtMoneyPlain(assets?.balance ?? 0)}
                </span>
              </button>

              {/* Notification bell placeholder */}
              <button
                aria-label="Notifications"
                className="w-11 h-11 rounded-full bg-[#16213e]/60 flex items-center justify-center active:bg-[#16213e]"
              >
                <Bell className="w-4 h-4 text-[#8892b0]" />
              </button>

              {/* User avatar */}
              <button
                onClick={() => router.push('/profile')}
                aria-label="View profile"
                className="w-11 h-11 rounded-full bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center active:scale-95 transition-transform"
              >
                <User className="w-4 h-4 text-white" />
              </button>
            </>
          ) : (
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                onClick={onLoginClick}
                className="text-[#ccd6f6] hover:text-[#f5a623] hover:bg-[#f5a623]/10 font-medium px-3 py-1.5 text-sm h-8"
              >
                Login
              </Button>
              <Button
                onClick={onRegisterClick}
                className="bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] hover:from-[#ffd700] hover:to-[#f5a623] font-semibold shadow-lg shadow-[#f5a623]/20 px-3 py-1.5 text-sm h-8"
              >
                Register
              </Button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
