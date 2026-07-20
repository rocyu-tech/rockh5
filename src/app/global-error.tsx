// Global error boundary — catches errors that occur in the root layout
// itself (above the route-level error.tsx). This is the "last resort"
// boundary — must render its own <html> + <body> because the root layout
// may have crashed.
//
// P0-3 FIX: without this, a render error in layout.tsx would show a
// blank white page with no recovery.

'use client';

import { useEffect } from 'react';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('[global error]', error);
  }, [error]);

  return (
    <html lang="en">
      <body style={{
        margin: 0,
        minHeight: '100vh',
        background: '#0a0a1a',
        color: 'white',
        fontFamily: 'system-ui, -apple-system, sans-serif',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '1rem',
      }}>
        <div style={{ maxWidth: '28rem', textAlign: 'center' }}>
          <div style={{
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: '4rem',
            height: '4rem',
            borderRadius: '50%',
            background: 'rgba(239, 68, 68, 0.1)',
            marginBottom: '1rem',
            fontSize: '2rem',
          }}>
            ⚠️
          </div>
          <h1 style={{ fontSize: '1.5rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>
            Application Error
          </h1>
          <p style={{ color: '#8892b0', fontSize: '0.875rem', marginBottom: '0.5rem' }}>
            A critical error occurred. Please reload the page.
          </p>
          {error.digest && (
            <p style={{ color: '#8892b0', fontSize: '0.75rem', marginBottom: '1.5rem', fontFamily: 'monospace' }}>
              Error ID: {error.digest}
            </p>
          )}
          <button
            onClick={reset}
            style={{
              background: '#f5a623',
              color: 'black',
              border: 'none',
              padding: '0.75rem 1.5rem',
              borderRadius: '0.5rem',
              fontWeight: 'bold',
              cursor: 'pointer',
            }}
          >
            Reload
          </button>
        </div>
      </body>
    </html>
  );
}
