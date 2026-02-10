# ---------------------------------------------------------
# Multi-stage Dockerfile for the Toposcope web UI (Next.js).
#
# Build:
#   docker build -t toposcope-web ./web
#
# Run:
#   docker run -p 3000:3000 toposcope-web
# ---------------------------------------------------------

# Stage 1 — install dependencies and build
FROM node:20-alpine AS builder

WORKDIR /app

# Enable corepack for pnpm
RUN corepack enable

# Copy package manifests first for better layer caching
COPY package.json pnpm-lock.yaml ./

RUN pnpm install --frozen-lockfile

# Copy source and build
COPY . .

# Build-time env vars
ARG NEXT_PUBLIC_API_MODE=hosted
ARG NEXT_PUBLIC_API_BASE_URL=
ENV NEXT_PUBLIC_API_MODE=${NEXT_PUBLIC_API_MODE}
ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}

RUN pnpm build

# Stage 2 — minimal production image
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production

# Don't run as root
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs

# Copy standalone output
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public

USER nextjs

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
