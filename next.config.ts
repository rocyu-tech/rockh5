import type { NextConfig } from "next";

// P0-10 FIX: previously this file contained multiple dangerous defaults:
//   - typescript.ignoreBuildErrors: true  → TS errors silently shipped to prod
//   - reactStrictMode: false              → masked effect clean-up bugs
//   - turbopack.root: "/data/src/rockh5"  → absolute path only valid on prod server
//   - allowedDevOrigins: ["*"]            → dev server accepts requests from any origin
//   - rewrites destination hard-coded to "http://47.108.78.147:8880" → prod IP in source
//
// All hard-coded values are now read from environment variables with safe
// defaults that work for local development. Production deployments MUST set
// BACKEND_URL via environment variable (see .env.example).

const BACKEND_URL =
  process.env.BACKEND_URL ?? "http://localhost:8880";

const nextConfig: NextConfig = {
  output: "standalone",

  // Re-enable TypeScript build-time checking. Any type error now fails the
  // build instead of silently shipping to production.
  // (Removed: typescript.ignoreBuildErrors: true)

  // Re-enable React strict mode to surface effect clean-up bugs in dev.
  reactStrictMode: true,

  // (Removed: turbopack.root hard-coded to "/data/src/rockh5")
  // Next.js auto-detects the project root; no override needed.

  // (Removed: allowedDevOrigins: ["*"])
  // If you need to access the dev server from another device on your LAN,
  // start next with `next dev -H 0.0.0.0` and access via your LAN IP.
  // Allowing "*" lets any website on the internet trigger HMR connections
  // to your dev server, which is a remote code execution risk.

  async rewrites() {
    // Proxy /api/v1/* to the backend. The backend URL is configurable via
    // the BACKEND_URL env var so dev/staging/prod can each point to their
    // own backend without changing source code.
    return [
      {
        source: "/api/v1/:path*",
        destination: `${BACKEND_URL}/api/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;
