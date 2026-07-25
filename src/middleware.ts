// Centralized route guard for protected pages.
//
// P0-9: previously each page implemented its own `if (!token) router.push('/')`
// check in a useEffect. This had 3 problems:
//   1. Inconsistent — some pages checked token, some checked `isLoggedIn`,
//      some rendered a stub. Behavior on the same auth state differed.
//   2. Late — the check ran AFTER the page rendered its protected UI,
//      causing a flash of unauthenticated content (FOUC).
//   3. Dead-end — redirect target was always `/` with no `?next=` param,
//      so after login the user landed on the home page instead of
//      returning to the page they tried to access.
//
// This middleware runs on the server before any page renders. It checks
// the `access_token` cookie (set by /auth/login) — if missing, redirects
// to /?next=<original-path>. The LoginModal reads `next` from
// `searchParams` and `router.replace`s there on success.
//
// Note: cookies are httpOnly, but Next.js middleware runs on the edge
// runtime which CAN read httpOnly cookies via `request.cookies.get()`.
// This is by design — the middleware is trusted server-side code.

import { NextResponse, type NextRequest } from 'next/server';

const PROTECTED_PREFIXES = [
  '/wallet',
  '/profile',
  '/play/',
  '/inventory',
  '/history',
  '/agent',
  '/mail',
  '/tasks',
  '/ranking',
  '/vip',
];

const PUBLIC_PATHS = ['/', '/forgot-password', '/games', '/promotions'];

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Skip non-page requests (API routes, _next static, etc.)
  if (
    pathname.startsWith('/_next/') ||
    pathname.startsWith('/api/') ||
    pathname.includes('.')
  ) {
    return NextResponse.next();
  }

  // Check if this path requires auth
  const isProtected = PROTECTED_PREFIXES.some((p) =>
    p === '/' ? pathname === '/' : pathname.startsWith(p)
  );

  if (!isProtected) {
    return NextResponse.next();
  }

  // Check for the access_token cookie.
  // This cookie is set client-side by auth store (syncTokenCookie) after
  // login/register/hydrate. It mirrors the JWT from localStorage so that
  // this server-side middleware can gate routes without reading localStorage.
  const accessToken = request.cookies.get('access_token')?.value;

  if (!accessToken) {
    // Not authenticated — redirect to / with ?next=<original-path>.
    // The login modal will read this and redirect back after success.
    const loginUrl = new URL('/', request.url);
    loginUrl.searchParams.set('next', pathname + request.nextUrl.search);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  // Run on all routes EXCEPT static assets and API endpoints (those are
  // handled by Next.js internally and don't need auth gating).
  matcher: [
    '/((?!_next/static|_next/image|favicon.ico|logo.svg|api/).*)',
  ],
};
