'use client';

import { Gift } from 'lucide-react';
import Navbar from '@/components/Navbar';
import PromotionsSection from '@/components/PromotionsSection';
import { useAuthStore } from '@/store/auth';

export default function PromotionsPage() {
  const { isLoggedIn } = useAuthStore();

  const handleSpinClick = () => {
    if (!isLoggedIn) {
      window.dispatchEvent(new CustomEvent('auth:logout'));
    } else {
      window.dispatchEvent(new CustomEvent('nav:open-spin'));
    }
  };

  return (
    <div>
      <Navbar
        onLoginClick={() => window.dispatchEvent(new CustomEvent('auth:logout'))}
        onRegisterClick={() => window.dispatchEvent(new CustomEvent('nav:open-register'))}
      />

      <main className="pt-14 px-4">
        {/* Page header */}
        <div className="flex items-center gap-2 mb-4">
          <Gift className="w-5 h-5 text-[#f5a623]" />
          <h1 className="text-lg font-bold text-white">Promotions</h1>
        </div>
        <p className="text-xs text-[#8892b0] mb-4">
          Take advantage of our exclusive bonuses and promotions. Boost your bankroll today!
        </p>

        {/* Promotions list */}
        <PromotionsSection onSpinClick={handleSpinClick} />
      </main>
    </div>
  );
}
