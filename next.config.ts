import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  typescript: {
    ignoreBuildErrors: true,
  },
  turbopack: {
    root: "/data/src/rockh5",
  },
  reactStrictMode: false,
  allowedDevOrigins: ["*"],
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: "http://47.108.78.147:8880/api/v1/:path*",
      },
    ];
  },
};

export default nextConfig;
