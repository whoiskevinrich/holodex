# syntax=docker/dockerfile:1

# The frontend and Go stages run on the build host's platform (no QEMU) and cross-compile:
# dist/ is architecture-independent and the Go build is CGO-free. Only the runtime stage
# runs per target platform.

# --- Stage 1: frontend build ---
FROM --platform=$BUILDPLATFORM node:26-slim AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci
COPY web/ ./
RUN npm run build        # outputs to web/dist

# --- Stage 2: Go build ---
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS backend
WORKDIR /app
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY --from=frontend /app/web/dist ./cmd/holodex/web/dist
COPY . .
# Declared here, not at the top of the stage: a build ARG enters every later RUN's cache key,
# so declaring it earlier would make `go mod download` run once per target platform.
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags production -o /out/holodex ./cmd/holodex

# --- Stage 3: runtime ---
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      ffmpeg \
      libimage-exiftool-perl \
      mkvtoolnix \
      ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=backend /out/holodex /usr/local/bin/holodex
# MKVToolNix converts paths and output through the locale; under the default
# C locale it cannot open a non-ASCII path and truncates non-ASCII output.
ENV DATA_PATH=/data \
    PORT=7800 \
    LANG=C.UTF-8
EXPOSE 7800 7801
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s \
    CMD ["/usr/local/bin/holodex", "-healthcheck"]
ENTRYPOINT ["holodex"]
