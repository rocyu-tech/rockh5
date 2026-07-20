# Multi-stage Dockerfile for rockh5 (Next.js 15 standalone output).
#
# P0-7 FIX: previously the only deployment path was `next dev` via PM2,
# which ships unminified JS, full source maps, and React dev warnings to
# production. This Dockerfile builds a production artifact using
# `next build` and serves it via `node server.js` from the standalone
# output directory.
#
# Build:
#   docker build -t rockh5:latest .
#
# Run:
#   docker run -p 8890:8890 \
#     -e BACKEND_URL=http://gate:8880 \
#     -e NEXT_PUBLIC_BACKEND_URL=http://gate:8880 \
#     rockh5:latest
#
# Image size: ~180MB (alpine + node + standalone build).

# ---- Stage 1: deps ----
FROM node:22-alpine AS deps
WORKDIR /app

# Install dependencies (use npm ci for reproducible builds).
COPY package.json package-lock.json* bun.lock* ./
RUN if [ -f package-lock.json ]; then \
      npm ci --no-audit --no-fund; \
    else \
      npm install --no-audit --no-fund; \
    fi

# ---- Stage 2: build ----
FROM node:22-alpine AS builder
WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Disable telemetry during build.
ENV NEXT_TELEMETRY_DISABLED=1

# Build the standalone artifact.
RUN npm run build

# ---- Stage 3: runtime ----
FROM node:22-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=8890
ENV HOSTNAME=0.0.0.0

# Create a non-root user for security.
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs

# Copy the standalone build + static assets + public folder.
# The standalone output is a self-contained Node.js server.
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nodejs /app/public ./public

# Health check — k8s/docker will hit /api/health every 30s.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8890/api/health || exit 1

USER nextjs

EXPOSE 8890

# Run the standalone server (faster than `next start`, no Next.js CLI overhead).
CMD ["node", "server.js"]
