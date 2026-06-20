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
  History,
} from 'lucide-react';

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

  const handleMobileAction = (action: string) => {
    setMobileOpen(false);
    setTimeout(() => {
      window.dispatchEvent(new CustomEvent(action));
    }, 200);
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
        <div className="flex items-center justify-between h-14 md:h-16">
          {/* Logo */}
          <div className="flex items-center gap-2 cursor-pointer" onClick={() => handleNavClick('home')}>
            <div className="w-8 h-8 md:w-10 md:h-10 rounded-lg bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center">
              <Gamepad2 className="w-5 h-5 md:w-6 md:h-6 text-white" />
            </div>
            <span className="text-lg md:text-xl font-bold text-gold-gradient tracking-tight">
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
          <div className="flex items-center gap-2">
            {isLoggedIn ? (
              <>
                {/* Balance - visible on all sizes */}
                <button
                  onClick={() => handleMobileAction('nav:open-transactions')}
                  className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-[#16213e] border border-[#f5a623]/20 active:bg-[#16213e]/80"
                >
                  <Wallet className="w-3.5 h-3.5 text-[#f5a623]" />
                  <span className="text-xs sm:text-sm font-semibold text-[#f5a623]">
                    {assets?.balance?.toLocaleString() ?? '0.00'}
                  </span>
                </button>

                {/* User Dropdown - desktop */}
                <div className="hidden sm:block">
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" className="flex items-center gap-2 px-3 hover:bg-white/5">
                        <div className="w-8 h-8 rounded-full bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center">
                          <User className="w-4 h-4 text-white" />
                        </div>
                        <ChevronDown className="w-3 h-3 text-[#8892b0]" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-56 bg-[#1a1a2e] border-[#f5a623]/20">
                      <DropdownMenuItem onSelect={() => window.dispatchEvent(new CustomEvent("nav:open-profile"))} className="text-[#ccd6f6] focus:bg-[#f5a623]/10 focus:text-[#f5a623] cursor-pointer py-3">
                        <User className="mr-2 h-4 w-4" />
                        Profile
                      </DropdownMenuItem>
                      <DropdownMenuItem onSelect={() => window.dispatchEvent(new CustomEvent("nav:open-transactions"))} className="text-[#ccd6f6] focus:bg-[#f5a623]/10 focus:text-[#f5a623] cursor-pointer py-3">
                        <TrendingUp className="mr-2 h-4 w-4" />
                        Transaction History
                      </DropdownMenuItem>
                      <DropdownMenuSeparator className="bg-[#f5a623]/10" />
                      <DropdownMenuItem onClick={logout} className="text-[#e94560] focus:bg-[#e94560]/10 focus:text-[#e94560] py-3">
                        <LogOut className="mr-2 h-4 w-4" />
                        Logout
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                {/* User avatar - mobile (tap opens sheet) */}
                <div className="sm:hidden">
                  <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
                    <SheetTrigger asChild>
                      <button className="w-9 h-9 rounded-full bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center active:scale-95 transition-transform">
                        <User className="w-4 h-4 text-white" />
                      </button>
                    </SheetTrigger>
                    <SheetContent side="right" className="w-72 bg-[#0a0a1a] border-[#f5a623]/20 p-0">
                      <SheetHeader className="sr-only">
                        <SheetTitle>Menu</SheetTitle>
                        <SheetDescription>Main navigation menu</SheetDescription>
                      </SheetHeader>
                      <div className="flex flex-col h-full">
                        {/* User info */}
                        <div className="p-5 border-b border-[#f5a623]/10">
                          <div className="flex items-center gap-3">
                            <div className="w-12 h-12 rounded-full bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center">
                              <User className="w-6 h-6 text-white" />
                            </div>
                            <div className="flex-1 min-w-0">
                              <p className="text-sm font-semibold text-white truncate">{user?.nickname || user?.email || 'Player'}</p>
                              <div className="flex items-center gap-1 mt-0.5">
                                <Gem className="w-3 h-3 text-[#f5a623]" />
                                <span className="text-xs text-[#f5a623]">VIP {user?.vip_level ?? 0}</span>
                              </div>
                            </div>
                          </div>
                          <div className="flex items-center gap-2 mt-3 px-3 py-2.5 rounded-lg bg-[#16213e]">
                            <Wallet className="w-4 h-4 text-[#f5a623]" />
                            <span className="text-sm font-semibold text-[#f5a623]">
                              {assets?.balance?.toLocaleString() ?? '0.00'} {assets?.currency ?? 'USD'}
                            </span>
                          </div>
                        </div>

                        {/* Nav items */}
                        <nav className="flex-1 p-3 space-y-1">
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

                        {/* Bottom actions */}
                        <div className="p-3 border-t border-[#f5a623]/10 space-y-1">
                          <button
                            onClick={() => handleMobileAction('nav:open-profile')}
                            className="flex items-center gap-3 w-full px-4 py-3 rounded-lg text-sm font-medium text-[#ccd6f6] hover:bg-[#f5a623]/10 active:bg-[#f5a623]/20"
                          >
                            <User className="w-5 h-5" />
                            Profile
                          </button>
                          <button
                            onClick={() => handleMobileAction('nav:open-transactions')}
                            className="flex items-center gap-3 w-full px-4 py-3 rounded-lg text-sm font-medium text-[#ccd6f6] hover:bg-[#f5a623]/10 active:bg-[#f5a623]/20"
                          >
                            <History className="w-5 h-5" />
                            Transaction History
                          </button>
                          <button
                            onClick={() => { handleMobileAction('auth:logout'); logout(); }}
                            className="flex items-center gap-3 w-full px-4 py-3 rounded-lg text-sm font-medium text-[#e94560] hover:bg-[#e94560]/10 active:bg-[#e94560]/20"
                          >
                            <LogOut className="w-5 h-5" />
                            Logout
                          </button>
                        </div>
                      </div>
                    </SheetContent>
                  </Sheet>
                </div>
              </>
            ) : (
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  onClick={onLoginClick}
                  className="text-[#ccd6f6] hover:text-[#f5a623] hover:bg-[#f5a623]/10 font-medium px-3 py-2"
                >
                  <LogIn className="w-4 h-4 sm:mr-1.5" />
                  <span className="hidden sm:inline">Login</span>
                </Button>
                <Button
                  onClick={onRegisterClick}
                  className="bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] hover:from-[#ffd700] hover:to-[#f5a623] font-semibold shadow-lg shadow-[#f5a623]/20 px-3 sm:px-4 py-2.5"
                >
                  <UserPlus className="w-4 h-4 sm:mr-1.5" />
                  <span className="hidden sm:inline">Register</span>
                </Button>
              </div>
            )}

            {/* Mobile hamburger - only when not logged in */}
            {!isLoggedIn && (
              <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
                <SheetTrigger asChild className="lg:hidden">
                  <Button variant="ghost" size="icon" className="text-[#ccd6f6] hover:bg-white/5">
                    <Menu className="w-5 h-5" />
                  </Button>
                </SheetTrigger>
                <SheetContent side="right" className="w-72 bg-[#0a0a1a] border-[#f5a623]/20 p-0">
                  <SheetHeader className="sr-only">
                    <SheetTitle>Menu</SheetTitle>
                    <SheetDescription>Main navigation menu</SheetDescription>
                  </SheetHeader>
                  <div className="flex flex-col h-full">
                    <div className="p-5 border-b border-[#f5a623]/10">
                      <span className="text-lg font-bold text-gold-gradient">RockGame</span>
                    </div>
                    <nav className="flex-1 p-3 space-y-1">
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
                    <div className="p-4 border-t border-[#f5a623]/10 space-y-2">
                      <Button
                        variant="outline"
                        onClick={() => { setMobileOpen(false); onLoginClick(); }}
                        className="w-full border-[#f5a623]/30 text-[#f5a623] hover:bg-[#f5a623]/10 py-3"
                      >
                        <LogIn className="w-4 h-4 mr-2" />
                        Login
                      </Button>
                      <Button
                        onClick={() => { setMobileOpen(false); onRegisterClick(); }}
                        className="w-full bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] py-3"
                      >
                        <UserPlus className="w-4 h-4 mr-2" />
                        Register
                      </Button>
                    </div>
                  </div>
                </SheetContent>
              </Sheet>
            )}
          </div>
        </div>
      </div>
    </header>
  );
}
