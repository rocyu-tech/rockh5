'use client';

// Age gate modal — shown once per browser (cookie-backed).
//
// P0-9 FIX: real-money gaming requires an explicit 18+ confirmation
// at first visit. The modal blocks all interaction until the user
// confirms. Declining redirects to a non-gaming exit page (Google).
//
// The cookie (rockgame_age_confirmed) expires after 365 days — after
// that, the user is prompted again. This is the standard pattern for
// Curacao / MGA licensed operators.
//
// Place <AgeGate /> at the root layout level so it covers every page.

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { Shield } from 'lucide-react';

const AGE_COOKIE = 'rockgame_age_confirmed';
const COOKIE_TTL_DAYS = 365;

export default function AgeGate() {
  const router = useRouter();
  const [show, setShow] = useState(false);

  useEffect(() => {
    // Only show on client-side render to avoid SSR hydration mismatch.
    if (typeof window === 'undefined') return;
    const cookies = document.cookie.split(';').map((c) => c.trim());
    const confirmed = cookies.find((c) => c.startsWith(`${AGE_COOKIE}=`));
    if (!confirmed) {
      setShow(true);
    }
  }, []);

  const handleAccept = () => {
    const expires = new Date(Date.now() + COOKIE_TTL_DAYS * 24 * 60 * 60 * 1000).toUTCString();
    document.cookie = `${AGE_COOKIE}=1; expires=${expires}; path=/; SameSite=Lax`;
    setShow(false);
  };

  const handleDecline = () => {
    // Redirect to a non-gaming exit page.
    window.location.href = 'https://www.google.com';
  };

  if (!show) return null;

  return (
    <div className="fixed inset-0 z-[100] bg-black/90 backdrop-blur-md flex items-center justify-center p-4">
      <div className="max-w-md w-full bg-[#1a1a2e] border border-[#f5a623]/30 rounded-2xl p-8 text-center">
        <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-[#f5a623]/10 mb-4">
          <Shield className="w-8 h-8 text-[#f5a623]" />
        </div>
        <h1 className="text-2xl font-bold text-white mb-2">
          🔞 Age Verification Required
        </h1>
        <p className="text-sm text-[#8892b0] mb-2">
          This website contains real-money gaming content and is restricted to persons aged 18 years and above (or the legal age of majority in your jurisdiction).
        </p>
        <p className="text-xs text-[#8892b0] mb-6">
          By clicking <strong className="text-white">&quot;I am 18+&quot;</strong>, you confirm that you meet the legal age requirement and accept our{' '}
          <Link href="/legal/terms" className="text-[#f5a623] hover:underline">Terms of Service</Link> and{' '}
          <Link href="/legal/privacy" className="text-[#f5a623] hover:underline">Privacy Policy</Link>.
        </p>
        <div className="space-y-3">
          <button
            onClick={handleAccept}
            className="w-full py-3 rounded-lg bg-gradient-to-r from-[#f5a623] to-[#e94560] text-black font-bold hover:opacity-90 transition-opacity"
          >
            I am 18+ and accept
          </button>
          <button
            onClick={handleDecline}
            className="w-full py-2 rounded-lg bg-[#0a0a1a] border border-white/10 text-[#8892b0] text-sm font-medium hover:bg-[#16213e] transition-colors"
          >
            I am under 18 — leave
          </button>
        </div>
        <p className="text-[10px] text-[#8892b0] mt-6">
          Gambling can be addictive. Please play responsibly.
          <br />
          <Link href="/legal/responsible-gaming" className="text-[#f5a623] hover:underline">Responsible Gaming</Link>
        </p>
      </div>
    </div>
  );
}
