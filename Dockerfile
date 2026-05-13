# ── Build Stage ──
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /plugin-hub .

# ── Runtime Stage ──
# Minimal Alpine with Python for isolated test execution
FROM alpine:3.21

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    python3 \
    py3-pip \
    && pip3 install --no-cache-dir pytest pytest-json-report --break-system-packages

COPY --from=builder /plugin-hub /usr/local/bin/plugin-hub
COPY config/plugins.yaml /etc/plugin-hub/plugins.yaml

EXPOSE 8000
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8000/health || exit 1

ENTRYPOINT ["/usr/local/bin/plugin-hub"]
CMD ["-config", "/etc/plugin-hub/plugins.yaml"]
