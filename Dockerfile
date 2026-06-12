# syntax=docker/dockerfile:1

# --- Stage 1: frontend build ---
FROM node:26-slim AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci
COPY web/ ./
RUN npm run build        # outputs to web/dist

# --- Stage 2: Go build ---
FROM golang:1.26-bookworm AS backend
WORKDIR /app
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY --from=frontend /app/web/dist ./cmd/holodex/web/dist
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -tags production -o /out/holodex ./cmd/holodex

# --- Stage 3: runtime ---
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ffmpeg \
      libimage-exiftool-perl \
      ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=backend /out/holodex /usr/local/bin/holodex
ENV DATA_PATH=/data \
    PORT=7800
EXPOSE 7800 7801
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s \
    CMD ["/usr/local/bin/holodex", "-healthcheck"]
ENTRYPOINT ["holodex"]
