import type { NextConfig } from "next";

const BACKEND_URL =
  process.env.BACKEND_URL ?? "http://localhost:8880";

const nextConfig: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  transpilePackages: ["@connectrpc/connect", "@connectrpc/connect-web", "@bufbuild/protobuf"],
  webpack: (config) => {
    config.resolve.extensionAlias = {
      ".js": [".ts", ".js"],
    };
    return config;
  },

  // P0-10: Security headers + CSP.
  // - frame-ancestors 'none' → blocks clickjacking (the admin panel
  //   and player app MUST NOT be iframeable).
  // - default-src 'self' → blocks external scripts/styles/images by default.
  // - connect-src 'self' ws: wss: → allows same-origin API + WebSocket.
  // - img-src 'self' data: https: → allows avatar/game cover images.
  // - script-src 'self' 'unsafe-inline' → Next.js inlines runtime chunks;
  //   for stricter CSP, add a nonce via middleware.
  // - style-src 'self' 'unsafe-inline' → Tailwind utility classes are inline.
  // - frame-ancestors 'none' → prevents the page from being embedded in
  //   an iframe (defense against clickjacking).
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          {
            key: "X-Frame-Options",
            value: "DENY",
          },
          {
            key: "X-Content-Type-Options",
            value: "nosniff",
          },
          {
            key: "Referrer-Policy",
            value: "strict-origin-when-cross-origin",
          },
          {
            key: "Permissions-Policy",
            value: "geolocation=(), microphone=(), camera=(), payment=(self)",
          },
          {
            key: "Strict-Transport-Security",
            value: "max-age=63072000; includeSubDomains; preload",
          },
          {
            key: "Content-Security-Policy",
            value: [
              "default-src 'self'",
              "script-src 'self' 'unsafe-inline' 'unsafe-eval'",
              "style-src 'self' 'unsafe-inline'",
              "img-src 'self' data: https:",
              "font-src 'self' data:",
              "connect-src 'self' ws: wss: https:",
              "frame-ancestors 'none'",
              "base-uri 'self'",
              "form-action 'self'",
            ].join("; "),
          },
        ],
      },
    ];
  },

  async rewrites() {
    return [
      // Connect/gRPC-Web RPC proxy — H5 sends binary protobuf to Gate
      {
        source: "/rockgame.:service/:method*",
        destination: `${BACKEND_URL}/rockgame.:service/:method*`,
      },
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
