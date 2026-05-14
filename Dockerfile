# ── Build Stage ──
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /plugin-hub .

# ── Runtime Stage ──
# Minimal Alpine — no Python/pytest needed (tests run in CI)
FROM alpine:3.21

RUN apk add --no-cache \
    ca-certificates \
    tzdata

COPY --from=builder /plugin-hub /usr/local/bin/plugin-hub

# Manifest URL is set via K8s env MANIFEST_URL

EXPOSE 8000
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8000/health || exit 1

ENTRYPOINT ["/usr/local/bin/plugin-hub"]
