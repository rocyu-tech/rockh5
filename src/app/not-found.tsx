// 404 page — shown when no route matches.
//
// P0-3 FIX: previously, hitting a non-existent URL showed the Next.js
// default gray 404 page. For a real-money app, a branded 404 is a minor
// but important piece of polish — users who typo a URL should land on
// something that looks intentional.

import Link from 'next/link';
import { Home, Gamepad2 } from 'lucide-react';

export default function NotFound() {
  return (
    <div className="min-h-screen bg-[#0a0a1a] text-white flex items-center justify-center p-4">
      <div className="max-w-md w-full text-center">
        <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-[#f5a623] to-[#e94560] mb-6">
          <Gamepad2 className="w-10 h-10 text-white" />
        </div>
        <h1 className="text-6xl font-bold mb-2 text-gold-gradient">404</h1>
        <h2 className="text-xl font-semibold mb-2">Page not found</h2>
        <p className="text-sm text-[#8892b0] mb-8">
          The page you&apos;re looking for doesn&apos;t exist or has been moved.
        </p>
        <div className="flex gap-3 justify-center">
          <Link
            href="/"
            className="px-6 py-3 rounded-lg bg-[#f5a623] text-black font-bold hover:opacity-90 transition-opacity flex items-center gap-2"
          >
            <Home className="w-4 h-4" /> Back to Home
          </Link>
          <Link
            href="/games"
            className="px-6 py-3 rounded-lg bg-[#1a1a2e] border border-white/10 text-white font-bold hover:border-[#f5a623]/30 transition-colors flex items-center gap-2"
          >
            <Gamepad2 className="w-4 h-4" /> Browse Games
          </Link>
        </div>
      </div>
    </div>
  );
}
