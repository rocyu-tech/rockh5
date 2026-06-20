'use client';

import { useState, useEffect } from 'react';
import Navbar from '@/components/Navbar';
import BannerCarousel from '@/components/BannerCarousel';
import GameCategories from '@/components/GameCategories';
import GameGrid from '@/components/GameGrid';
import VIPSection from '@/components/VIPSection';
import PromotionsSection from '@/components/PromotionsSection';
import SpinWheel from '@/components/SpinWheel';
import Footer from '@/components/Footer';
import LoginModal from '@/components/LoginModal';
import RegisterModal from '@/components/RegisterModal';
import ConnectionBanner from '@/components/ConnectionBanner';
import ProfileModal from '@/components/ProfileModal';
import { useAuthStore } from '@/store/auth';
import { useApiStatus, ApiStatusContext } from '@/lib/api-status';
import { Gem, Gamepad2, Gift, TrendingUp, Users, Trophy, Star, Shield } from 'lucide-react';


export default function Home() {
  const [activeSection, setActiveSection] = useState('home');
  const [activeCategory, setActiveCategory] = useState(0);
  const [loginOpen, setLoginOpen] = useState(false);
  const [registerOpen, setRegisterOpen] = useState(false);
  const [spinOpen, setSpinOpen] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const { hydrate, isLoggedIn } = useAuthStore();
  const apiStatus = useApiStatus();

  useEffect(() => {
    hydrate();

    // Listen for auth logout events from API interceptor
    const handleAuthLogout = () => {
      setLoginOpen(true);
    };
    window.addEventListener('auth:logout', handleAuthLogout);

    // Listen for nav events from Navbar
    const handleOpenProfile = () => setProfileOpen(true);
    window.addEventListener('nav:open-profile', handleOpenProfile);
    return () => {
      window.removeEventListener('auth:logout', handleAuthLogout);
      window.removeEventListener('nav:open-profile', handleOpenProfile);
    };
    return () => window.removeEventListener('auth:logout', handleAuthLogout);
  }, [hydrate]);

  // Scroll spy for active section
  useEffect(() => {
    const handleScroll = () => {
      const sections = ['agent', 'promotions', 'vip', 'games', 'home'];
      for (const sectionId of sections) {
        const el = document.getElementById(sectionId);
        if (el) {
          const rect = el.getBoundingClientRect();
          if (rect.top <= 150) {
            setActiveSection(sectionId);
            break;
          }
        }
      }
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const switchToRegister = () => {
    setLoginOpen(false);
    setTimeout(() => setRegisterOpen(true), 200);
  };

  const switchToLogin = () => {
    setRegisterOpen(false);
    setTimeout(() => setLoginOpen(true), 200);
  };

  const handleSpinClick = () => {
    if (!isLoggedIn) {
      setLoginOpen(true);
    } else {
      setSpinOpen(true);
    }
  };

  return (
    <ApiStatusContext.Provider value={apiStatus}>
    <div className={`min-h-screen flex flex-col bg-[#0a0a1a] ${apiStatus.isOffline ? 'pt-10' : ''}`}>
      {/* Background decoration */}
      <div className="fixed inset-0 pointer-events-none overflow-hidden">
        <div className="absolute -top-40 -right-40 w-80 h-80 rounded-full bg-[#f5a623]/3 blur-[100px]" />
        <div className="absolute top-1/3 -left-40 w-96 h-96 rounded-full bg-[#e94560]/3 blur-[120px]" />
        <div className="absolute bottom-1/4 right-1/4 w-64 h-64 rounded-full bg-[#4ecdc4]/2 blur-[80px]" />
      </div>

      <Navbar
        activeSection={activeSection}
        onSectionChange={setActiveSection}
        onLoginClick={() => setLoginOpen(true)}
        onRegisterClick={() => setRegisterOpen(true)}
      />

      <main className="flex-1 relative z-10">
        {/* Hero / Banner */}
        <div id="home" className="pt-4 md:pt-6">
          <BannerCarousel />

          {/* Stats bar */}
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 mt-6">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4">
              {[
                { icon: Gamepad2, label: 'Games', value: '5,000+', color: '#f5a623' },
                { icon: Users, label: 'Players', value: '100K+', color: '#e94560' },
                { icon: Trophy, label: 'Jackpots', value: '$10M+', color: '#4ecdc4' },
                { icon: Shield, label: 'Licensed', value: '100%', color: '#a855f7' },
              ].map((stat) => (
                <div
                  key={stat.label}
                  className="flex items-center gap-3 p-3 md:p-4 rounded-xl bg-[#1a1a2e]/60 border border-[#f5a623]/10 backdrop-blur-sm"
                >
                  <div
                    className="w-10 h-10 rounded-lg flex items-center justify-center"
                    style={{ backgroundColor: `${stat.color}15` }}
                  >
                    <stat.icon className="w-5 h-5" style={{ color: stat.color }} />
                  </div>
                  <div>
                    <p className="text-xs text-[#8892b0]">{stat.label}</p>
                    <p className="text-sm md:text-base font-bold text-white">{stat.value}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Games Section */}
        <section id="games" className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 mt-10 md:mt-14">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h2 className="text-xl md:text-2xl font-bold text-white flex items-center gap-2">
                <Gamepad2 className="w-6 h-6 text-[#f5a623]" />
                Game Lobby
              </h2>
              <p className="text-sm text-[#8892b0] mt-1">Discover thousands of premium games</p>
            </div>
          </div>

          <div className="space-y-6">
            <GameCategories
              activeCategory={activeCategory}
              onCategoryChange={setActiveCategory}
            />
            <GameGrid categoryId={activeCategory} />
          </div>
        </section>

        {/* VIP Section */}
        <section id="vip" className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 mt-16 md:mt-20">
          <div className="flex items-center gap-2 mb-6">
            <Gem className="w-6 h-6 text-[#f5a623]" />
            <h2 className="text-xl md:text-2xl font-bold text-white">VIP Club</h2>
          </div>
          <p className="text-sm text-[#8892b0] mb-6">
            Unlock exclusive rewards and privileges. The more you play, the higher you climb.
          </p>
          <VIPSection />
        </section>

        {/* Promotions Section */}
        <section id="promotions" className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 mt-16 md:mt-20">
          <div className="flex items-center gap-2 mb-6">
            <Gift className="w-6 h-6 text-[#f5a623]" />
            <h2 className="text-xl md:text-2xl font-bold text-white">Promotions</h2>
          </div>
          <p className="text-sm text-[#8892b0] mb-6">
            Take advantage of our exclusive bonuses and promotions. Boost your bankroll today!
          </p>
          <PromotionsSection onSpinClick={handleSpinClick} />
        </section>

        {/* Agent Section */}
        <section id="agent" className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 mt-16 md:mt-20">
          <div className="rounded-2xl overflow-hidden border border-[#f5a623]/20">
            <div className="relative p-8 md:p-12 bg-gradient-to-r from-[#1a1a2e] via-[#16213e] to-[#1a1a2e]">
              {/* Decorative elements */}
              <div className="absolute top-0 right-0 w-64 h-64 bg-[#f5a623]/5 rounded-full blur-[60px]" />
              <div className="absolute bottom-0 left-0 w-48 h-48 bg-[#e94560]/5 rounded-full blur-[60px]" />

              <div className="relative z-10 flex flex-col md:flex-row items-center gap-6 md:gap-12">
                <div className="flex-shrink-0">
                  <div className="w-20 h-20 md:w-24 md:h-24 rounded-2xl bg-gradient-to-br from-[#f5a623]/20 to-[#e94560]/20 border border-[#f5a623]/20 flex items-center justify-center">
                    <TrendingUp className="w-10 h-10 md:w-12 md:h-12 text-[#f5a623]" />
                  </div>
                </div>
                <div className="text-center md:text-left flex-1">
                  <h2 className="text-2xl md:text-3xl font-bold text-white mb-2">
                    Become an <span className="text-gold-gradient">Agent</span>
                  </h2>
                  <p className="text-sm md:text-base text-[#8892b0] max-w-lg">
                    Join our affiliate program and earn up to 45% commission. Build your network, track your earnings in real-time, and receive unlimited bonuses.
                  </p>
                </div>
                <div className="flex flex-col sm:flex-row gap-3">
                  <button className="px-6 py-2.5 bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold rounded-lg hover:from-[#ffd700] hover:to-[#f5a623] transition-all shadow-lg shadow-[#f5a623]/20 text-sm">
                    Join Now
                  </button>
                  <button className="px-6 py-2.5 border border-[#f5a623]/30 text-[#f5a623] font-semibold rounded-lg hover:bg-[#f5a623]/10 transition-all text-sm">
                    Learn More
                  </button>
                </div>
              </div>
            </div>
          </div>
        </section>
      </main>

      <Footer />

      <ConnectionBanner
        isOffline={apiStatus.isOffline}
        failedCount={apiStatus.failedCount}
        status={apiStatus.status}
        onDismiss={() => {}}
      />

      {/* Auth Modals */}
      <LoginModal
        open={loginOpen}
        onOpenChange={setLoginOpen}
        switchToRegister={switchToRegister}
      />
      <RegisterModal
        open={registerOpen}
        onOpenChange={setRegisterOpen}
        switchToLogin={switchToLogin}
      />

      {/* Spin Wheel Modal */}
      <SpinWheel
        open={spinOpen}
        onOpenChange={setSpinOpen}
      />
    </div>
    </ApiStatusContext.Provider>
  );
}
