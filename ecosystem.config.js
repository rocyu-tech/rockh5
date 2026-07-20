// PM2 config for rockh5.
//
// P0-2 FIX: was `next dev` (dev server) which ships:
//   - unminified JS (slow)
//   - full source maps (XSS surface — exposes SSR code paths)
//   - React dev-mode warnings splashed in console
//   - ~10× slower request handling
//
// Now uses `next start` after `next build`. The deployment script
// (manage.sh or CI) must run `npm run build` before starting PM2.
//
// For Docker deployments, prefer the Dockerfile (uses the standalone
// server.js output, even smaller + faster than next start).
//
// Usage:
//   npm run build
//   pm2 start ecosystem.config.js --env production

module.exports = {
  apps: [
    {
      name: "rockh5",
      // P0-2: changed from `npx next dev` to `next start` (uses built .next/).
      script: "node_modules/.bin/next",
      args: "start -p 8890",
      instances: 1,
      exec_mode: "fork",
      // P0-2: disabled watch mode — it restarts on file changes which is
      // fine for dev but causes WS connection drops in production.
      watch: false,
      env: {
        NODE_ENV: "production",
        PORT: 8890,
      },
      // P0-2: graceful shutdown — wait up to 10s for in-flight requests.
      kill_timeout: 10000,
      // Auto-restart on crash (max 10 restarts in 1 min to prevent loops).
      max_restarts: 10,
      min_uptime: "10s",
    },
  ],
};
