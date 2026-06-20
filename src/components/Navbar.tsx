'use client';

import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetTrigger } from '@/components/ui/sheet';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Menu,
  X,
  Wallet,
  LogIn,
  UserPlus,
  User,
  LogOut,
  ChevronDown,
  Gem,
  Home,
  Gamepad2,
  Gift,
  Users,
  TrendingUp,
} from 'lucide-react';
import Link from 'next/link';

interface NavbarProps {
  activeSection: string;
  onSectionChange: (section: string) => void;
  onLoginClick: () => void;
  onRegisterClick: () => void;
}

const navItems = [
  { id: 'home', label: 'Home', icon: Home },
  { id: 'games', label: 'Games', icon: Gamepad2 },
  { id: 'vip', label: 'VIP', icon: Gem },
  { id: 'promotions', label: 'Promotions', icon: Gift },
  { id: 'agent', label: 'Agent', icon: Users },
];

export default function Navbar({ activeSection, onSectionChange, onLoginClick, onRegisterClick }: NavbarProps) {
  const { isLoggedIn, user, assets, logout } = useAuthStore();
  const [scrolled, setScrolled] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    const handleScroll = () => setScrolled(window.scrollY > 20);
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const handleNavClick = (sectionId: string) => {
    onSectionChange(sectionId);
    setMobileOpen(false);
    const el = document.getElementById(sectionId);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };

  return (
    <header
      className={`fixed top-0 left-0 right-0 z-50 transition-all duration-300 ${
        scrolled
          ? 'bg-[#0a0a1a]/95 backdrop-blur-md border-b border-[#f5a623]/20 shadow-lg shadow-black/20'
          : 'bg-transparent'
      }`}
    >
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16 md:h-18">
          {/* Logo */}
          <div className="flex items-center gap-2 cursor-pointer" onClick={() => handleNavClick('home')}>
            <div className="w-8 h-8 md:w-10 md:h-10 rounded-lg bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center">
              <Gamepad2 className="w-5 h-5 md:w-6 md:h-6 text-white" />
            </div>
            <span className="text-xl md:text-2xl font-bold text-gold-gradient tracking-tight">
              RockGame
            </span>
          </div>

          {/* Desktop Nav */}
          <nav className="hidden lg:flex items-center gap-1">
            {navItems.map((item) => (
              <button
                key={item.id}
                onClick={() => handleNavClick(item.id)}
                className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200 ${
                  activeSection === item.id
                    ? 'text-[#f5a623] bg-[#f5a623]/10'
                    : 'text-[#8892b0] hover:text-[#e0e0e0] hover:bg-white/5'
                }`}
              >
                <item.icon className="w-4 h-4" />
                {item.label}
              </button>
            ))}
          </nav>

          {/* Right side */}
          <div className="flex items-center gap-2 md:gap-3">
            {isLoggedIn ? (
              <>
                {/* Balance */}
                <div className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[#16213e] border border-[#f5a623]/20">
                  <Wallet className="w-4 h-4 text-[#f5a623]" />
                  <span className="text-sm font-semibold text-[#f5a623]">
                    {assets?.balance?.toLocaleString() ?? '0.00'}
                  </span>
                  <span className="text-xs text-[#8892b0]">{assets?.currency ?? 'USD'}</span>
                </div>

                {/* User Dropdown */}
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" className="flex items-center gap-2 px-3 hover:bg-white/5">
                      <div className="w-8 h-8 rounded-full bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center">
                        <User className="w-4 h-4 text-white" />
                      </div>
                      <span className="hidden md:block text-sm text-[#ccd6f6] max-w-[100px] truncate">
                        {user?.nickname || user?.email || 'Player'}
                      </span>
                      <ChevronDown className="w-3 h-3 text-[#8892b0]" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent
                    align="end"
                    className="w-56 bg-[#1a1a2e] border-[#f5a623]/20"
                  >
                    <DropdownMenuItem onSelect={() => {
                      const evt = new CustomEvent("nav:open-profile");
                      window.dispatchEvent(evt);
                    }} className="text-[#ccd6f6] focus:bg-[#f5a623]/10 focus:text-[#f5a623] cursor-pointer">
                      <User className="mr-2 h-4 w-4" />
                      Profile
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => {
                      const evt = new CustomEvent("nav:open-transactions");
                      window.dispatchEvent(evt);
                    }} className="text-[#ccd6f6] focus:bg-[#f5a623]/10 focus:text-[#f5a623] cursor-pointer">
                      <TrendingUp className="mr-2 h-4 w-4" />
                      Transaction History
                    </DropdownMenuItem>
                    <DropdownMenuSeparator className="bg-[#f5a623]/10" />
                    <DropdownMenuItem
                      onClick={logout}
                      className="text-[#e94560] focus:bg-[#e94560]/10 focus:text-[#e94560]"
                    >
                      <LogOut className="mr-2 h-4 w-4" />
                      Logout
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </>
            ) : (
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  onClick={onLoginClick}
                  className="text-[#ccd6f6] hover:text-[#f5a623] hover:bg-[#f5a623]/10 font-medium"
                >
                  <LogIn className="w-4 h-4 mr-1.5" />
                  <span className="hidden sm:inline">Login</span>
                </Button>
                <Button
                  onClick={onRegisterClick}
                  className="bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] hover:from-[#ffd700] hover:to-[#f5a623] font-semibold shadow-lg shadow-[#f5a623]/20"
                >
                  <UserPlus className="w-4 h-4 mr-1.5" />
                  <span className="hidden sm:inline">Register</span>
                </Button>
              </div>
            )}

            {/* Mobile Menu */}
            <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
              <SheetTrigger asChild className="lg:hidden">
                <Button variant="ghost" size="icon" className="text-[#ccd6f6] hover:bg-white/5">
                  {mobileOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
                </Button>
              </SheetTrigger>
              <SheetContent side="right" className="w-72 bg-[#0a0a1a] border-[#f5a623]/20 p-0">
                <SheetHeader className="sr-only">
                  <SheetTitle>Navigation</SheetTitle>
                  <SheetDescription>Main navigation menu</SheetDescription>
                </SheetHeader>
                <div className="flex flex-col h-full">
                  <div className="p-6 border-b border-[#f5a623]/10">
                    <span className="text-xl font-bold text-gold-gradient">RockGame</span>
                  </div>
                  <nav className="flex-1 p-4 space-y-1">
                    {navItems.map((item) => (
                      <button
                        key={item.id}
                        onClick={() => handleNavClick(item.id)}
                        className={`flex items-center gap-3 w-full px-4 py-3 rounded-lg text-sm font-medium transition-all ${
                          activeSection === item.id
                            ? 'text-[#f5a623] bg-[#f5a623]/10'
                            : 'text-[#8892b0] hover:text-[#e0e0e0] hover:bg-white/5'
                        }`}
                      >
                        <item.icon className="w-5 h-5" />
                        {item.label}
                      </button>
                    ))}
                  </nav>
                  {isLoggedIn && (
                    <div className="p-4 border-t border-[#f5a623]/10">
                      <div className="flex items-center gap-3 px-4 py-3 rounded-lg bg-[#16213e]">
                        <Wallet className="w-4 h-4 text-[#f5a623]" />
                        <span className="text-sm font-semibold text-[#f5a623]">
                          {assets?.balance?.toLocaleString() ?? '0.00'} {assets?.currency ?? 'USD'}
                        </span>
                      </div>
                    </div>
                  )}
                </div>
              </SheetContent>
            </Sheet>
          </div>
        </div>
      </div>
    </header>
  );
}
