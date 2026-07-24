// Global error boundary — catches uncaught errors in any route.
//
// P0-3 FIX: previously, an uncaught error during render would show the
// Next.js default gray traceback (in dev) or a blank "Application error"
// message (in prod). For a real-money app, a white screen = lost bets +
// support load. This boundary shows a branded error page with a "Reload"
// button + a unique error ID for support reference.
//
// Note: this is the route-level boundary. For top-level (root layout)
// errors, see global-error.tsx.

'use client';

import { useEffect } from 'react';
import { Button } from '@/components/ui/button';
import { AlertTriangle, RefreshCw, Home } from 'lucide-react';

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Log to console — in prod this should also be sent to Sentry.
    console.error('[route error]', error);
  }, [error]);

  return (
    <div className="min-h-screen bg-[#0a0a1a] text-white flex items-center justify-center p-4">
      <div className="max-w-md w-full text-center">
        <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-red-500/10 mb-4">
          <AlertTriangle className="w-8 h-8 text-red-400" />
        </div>
        <h1 className="text-2xl font-bold mb-2">Something went wrong</h1>
        <p className="text-sm text-[#8892b0] mb-2">
          An unexpected error occurred. Please try again.
        </p>
        {error.digest && (
          <p className="text-xs text-[#8892b0] mb-6 font-mono">
            Error ID: {error.digest}
          </p>
        )}
        <div className="flex gap-3 justify-center">
          <Button
            onClick={reset}
            className="bg-[#f5a623] text-black hover:opacity-90"
          >
            <RefreshCw className="w-4 h-4 mr-2" /> Try again
          </Button>
          <Button
            variant="outline"
            onClick={() => window.location.href = '/'}
            className="border-white/20"
          >
            <Home className="w-4 h-4 mr-2" /> Home
          </Button>
        </div>
      </div>
    </div>
  );
}
