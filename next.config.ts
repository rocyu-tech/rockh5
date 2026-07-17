import type { NextConfig } from "next";

const BACKEND_URL =
  process.env.BACKEND_URL ?? "http://localhost:8880";

const nextConfig: NextConfig = {
  output: "standalone",
  reactStrictMode: true,

  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: `${BACKEND_URL}/api/v1/:path*`,
      },
      // P0: WebSocket proxy — Next.js rewrites support WS upgrade.
      // Client connects to /ws/v1/game/* which is proxied to the gate.
      {
        source: "/ws/v1/:path*",
        destination: `${BACKEND_URL}/ws/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;
