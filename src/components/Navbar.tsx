'use client';

import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/button';
import { Wallet, LogIn, UserPlus, User, LogOut, Gamepad2, Bell } from 'lucide-react';

interface NavbarProps {
  onLoginClick: () => void;
  onRegisterClick: () => void;
}

export default function Navbar({ onLoginClick, onRegisterClick }: NavbarProps) {
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
              <button onClick={() => { if (typeof window !== 'undefined') window.location.href = '/wallet'; }} className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-[#16213e] border border-[#f5a623]/20 active:bg-[#16213e]/80">
                <Wallet className="w-3.5 h-3.5 text-[#f5a623]" />
                <span className="text-xs font-semibold text-[#f5a623]">
                  {assets?.balance?.toLocaleString() ?? '0.00'}
                </span>
              </button>

              {/* Notification bell placeholder */}
              <button className="w-8 h-8 rounded-full bg-[#16213e]/60 flex items-center justify-center active:bg-[#16213e]">
                <Bell className="w-4 h-4 text-[#8892b0]" />
              </button>

              {/* User avatar */}
              <button
                onClick={() => {
                  // Navigate to profile page - using router is tricky here,
                  // so dispatch event for page.tsx to handle, or use a callback
                  if (typeof window !== 'undefined') {
                    window.location.href = '/profile';
                  }
                }}
                className="w-8 h-8 rounded-full bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center active:scale-95 transition-transform"
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
