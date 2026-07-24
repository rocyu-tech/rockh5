// Health check endpoint for k8s liveness/readiness probes + load balancer.
//
// P0-12 FIX: k8s probes need a real HTTP endpoint that returns 200
// when the app is ready to serve traffic. The previous /api/route.ts
// returned a hardcoded "Hello, world!" — not suitable for monitoring.
//
// Returns build metadata + uptime for ops debugging.

import { NextResponse } from 'next/server';

export const runtime = 'nodejs';
export const dynamic = 'force-dynamic';

const startedAt = Date.now();

export async function GET() {
  return NextResponse.json({
    status: 'ok',
    service: 'rockh5',
    version: process.env.npm_package_version || '0.1.0',
    build_id: process.env.BUILD_ID || null,
    commit: process.env.GIT_COMMIT || null,
    uptime_seconds: Math.floor((Date.now() - startedAt) / 1000),
    timestamp: new Date().toISOString(),
  }, {
    status: 200,
    headers: {
      'Cache-Control': 'no-store, no-cache, must-revalidate',
    },
  });
}
