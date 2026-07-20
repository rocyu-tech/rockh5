'use client';

import { useState, useEffect, useCallback } from 'react';
import { usePathname } from 'next/navigation';
import { useApiStatus, ApiStatusContext } from '@/lib/api-status';
import { useAuthStore } from '@/store/auth';
import LoginModal from '@/components/LoginModal';
import RegisterModal from '@/components/RegisterModal';
import SpinWheel from '@/components/SpinWheel';
import ConnectionBanner from '@/components/ConnectionBanner';
export default function AppProvider({ children }: { children: React.ReactNode }) {
  const [loginOpen, setLoginOpen] = useState(false);
  const [registerOpen, setRegisterOpen] = useState(false);
  const [spinOpen, setSpinOpen] = useState(false);
  const { hydrate, isLoggedIn, fetchUnreadMailCount } = useAuthStore();
  const apiStatus = useApiStatus();
  const pathname = usePathname();
  // P0: when on a full-screen game page, drop the `pb-14` bottom padding
  // that reserves space for BottomNav (BottomNav returns null on /play/*).
  const isGameRoute = pathname?.startsWith('/play/') ?? false;

  useEffect(() => {
    hydrate();
  }, [hydrate]);

  // Poll unread mail count (single source of truth in auth store)
  useEffect(() => {
    if (!isLoggedIn) return;
    fetchUnreadMailCount();
    const timer = setInterval(fetchUnreadMailCount, 30000);
    return () => clearInterval(timer);
  }, [isLoggedIn, fetchUnreadMailCount]);

  const handleAuthLogout = useCallback(() => {
    // Only show login modal if not already logged in
    // This prevents re-opening the login modal right after successful login
    // when a delayed 401 response triggers auth:logout
    const { isLoggedIn: currentlyLoggedIn } = useAuthStore.getState();
    if (!currentlyLoggedIn) {
      setLoginOpen(true);
    }
  }, []);

  const handleOpenSpin = useCallback(() => {
    const { isLoggedIn: currentlyLoggedIn } = useAuthStore.getState();
    if (currentlyLoggedIn) {
      setSpinOpen(true);
    } else {
      setLoginOpen(true);
    }
  }, []);

  const handleOpenRegister = useCallback(() => setRegisterOpen(true), []);

  useEffect(() => {
    window.addEventListener('auth:logout', handleAuthLogout);
    window.addEventListener('nav:open-spin', handleOpenSpin);
    window.addEventListener('nav:open-register', handleOpenRegister);

    return () => {
      window.removeEventListener('auth:logout', handleAuthLogout);
      window.removeEventListener('nav:open-spin', handleOpenSpin);
      window.removeEventListener('nav:open-register', handleOpenRegister);
    };
  }, [handleAuthLogout, handleOpenSpin, handleOpenRegister]);

  const switchToRegister = () => {
    setLoginOpen(false);
    setTimeout(() => setRegisterOpen(true), 200);
  };

  const switchToLogin = () => {
    setRegisterOpen(false);
    setTimeout(() => setLoginOpen(true), 200);
  };

  return (
    <ApiStatusContext.Provider value={apiStatus}>
      <div
        className={`min-h-screen flex flex-col bg-[#0a0a1a] ${isGameRoute ? '' : 'pb-[calc(3.5rem+env(safe-area-inset-bottom))]'} ${apiStatus.isOffline ? 'pt-10' : ''}`}
        style={{ paddingTop: isGameRoute ? 'env(safe-area-inset-top)' : undefined }}
      >
        {/* Background decoration */}
        <div className="fixed inset-0 pointer-events-none overflow-hidden">
          <div className="absolute -top-40 -right-40 w-80 h-80 rounded-full bg-[#f5a623]/3 blur-[100px]" />
          <div className="absolute top-1/3 -left-40 w-96 h-96 rounded-full bg-[#e94560]/3 blur-[120px]" />
          <div className="absolute bottom-1/4 right-1/4 w-64 h-64 rounded-full bg-[#4ecdc4]/2 blur-[80px]" />
        </div>

        <div className="flex-1 relative z-10">
          {children}
        </div>

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
