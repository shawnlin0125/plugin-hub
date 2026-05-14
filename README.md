# Plugin Hub

Universal plugin management platform — API Gateway + Admin Dashboard.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   hub.shawnlin.online                        │
│                                                              │
│  ┌──────────────────┐    ┌──────────────────────────────┐   │
│  │  Hub Admin (Go)   │    │  Ticket Proxy (Python)        │   │
│  │                    │    │                               │   │
│  │  Dashboard         │    │  /api/v1/{vendor}/search      │   │
│  │  Plugin Registry   │◀──▶│  /api/v1/{vendor}/orders     │   │
│  │  Vendor Assignment │    │  /api/v1/{vendor}/inventory   │   │
│  │  Reverse Proxy ────┼────▶  (proxy-high / proxy-normal) │   │
│  └──────────────────┘    └──────────────────────────────┘   │
│           │                                                   │
│           │ reads manifest from                               │
│           ▼                                                   │
│  ┌──────────────────────────────────────┐                   │
│  │  ticket-vendor/plugin-manifest.json   │                   │
│  │  (GitHub raw, CI auto-updates)        │                   │
│  └──────────────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────┘
```

## Components

| Component | Repo | Language | Image |
|-----------|------|:--------:|-------|
| Hub Admin | plugin-hub | Go | 28MB |
| Ticket Proxy | ticket-vendor | Python (FastAPI) | ~120MB |
| Vendor Plugins | ticket-vendor | Python | bundled in proxy |

## Quick Start

```bash
# Build Hub Admin
docker build -t plugin-hub:go-latest .

# Deploy to k3s
kubectl apply -f k8s/
```

## API

### Admin API (Hub manages plugins)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Platform health |
| GET | `/api/plugins` | List all discovered plugins |
| GET | `/api/plugins/{id}` | Plugin details |
| POST | `/api/plugins/{id}/enable` | Enable plugin (CI must pass) |
| POST | `/api/plugins/{id}/disable` | Disable plugin |
| GET | `/api/assignments` | List vendor → deployment mappings |
| POST | `/api/assignments/{vendor}` | Assign vendor to deployment |

### Business API (proxied to Ticket Proxy)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/{vendor}/search` | Search events |
| POST | `/api/v1/{vendor}/orders` | Create order |
| GET | `/api/v1/{vendor}/orders/{id}` | Get order |
| GET | `/api/v1/{vendor}/orders/{id}/poll` | Poll order |
| GET | `/api/v1/{vendor}/inventory` | Check inventory |

## Adding a New Vendor

1. Add plugin in [ticket-vendor](https://github.com/shawnlin0125/ticket-vendor) repo
2. CI passes → `plugin-manifest.json` auto-updates
3. Hub auto-discovers new vendor (≤ 5 min)
4. Dashboard: assign to proxy-high or proxy-normal
5. Update the proxy ConfigMap (via ArgoCD or manual)
6. Click Enable → vendor goes live

**Hub code never changes when adding vendors.**

## K8s Deployments

| Deployment | Replicas | Purpose |
|------------|:--------:|---------|
| hub-admin | 1 | Dashboard + API + reverse proxy |
| ticket-proxy-high | 2 | High-traffic vendor plugins |
| ticket-proxy-normal | 1 | Normal-traffic vendor plugins |

## Auto-Discovery

Hub fetches `plugin-manifest.json` from ticket-vendor's GitHub raw URL every 5 minutes.
Test results come from CI, not from the Hub.
