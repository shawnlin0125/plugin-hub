# Plugin Hub

Universal plugin management platform — API Gateway + Admin Dashboard.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   hub.shawnlin.online                        │
│                                                              │
│  ┌──────────────────┐    ┌──────────────────────────────┐   │
│  │  Hub Admin (Go)   │    │  Ticket Proxy (Python)        │   │
│  │                    │    │  × N replicas (all identical) │   │
│  │  Dashboard         │    │                               │   │
│  │  Plugin Registry   │◀──▶│  ALL enabled vendors loaded   │   │
│  │  Enable/Disable    │    │  /api/v1/{vendor}/search      │   │
│  │  Reverse Proxy ────┼────▶  /api/v1/{vendor}/orders     │   │
│  └──────────────────┘    │  /api/v1/{vendor}/inventory   │   │
│           │              └──────────────────────────────┘   │
│           │ reads manifest from        ▲                     │
│           ▼                            │ LOAD_PLUGINS        │
│  ┌──────────────────────────────────────┐ via ConfigMap     │
│  │  ticket-vendor/plugin-manifest.json   │                   │
│  │  (GitHub raw, CI auto-updates)        │                   │
│  └──────────────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────┘
```

## Key Design Principle

**No capacity pools. No per-vendor deployment. One unified proxy fleet.**

- A single `ticket-proxy` Deployment serves ALL enabled vendors
- When admin enables a vendor in the dashboard → every proxy pod loads it
- Scale replicas horizontally for capacity — all pods are identical
- Hub reverse-proxies `/api/v1/{vendor}/*` → `ticket-proxy` service → any pod handles any vendor

```
Multi-server scenario:
  Server A: ticket-proxy (replica 1) ─┐
  Server B: ticket-proxy (replica 2) ─┼─ same ConfigMap, same LOAD_PLUGINS
  Server C: ticket-proxy (replica 3) ─┘
```

## Components

| Component | Repo | Language | Image |
|-----------|------|:--------:|-------|
| Hub Admin | plugin-hub | Go | ~8MB |
| Ticket Proxy | ticket-vendor | Python (FastAPI) | ~120MB |
| Vendor Plugins | ticket-vendor | Python | bundled in proxy |

## Quick Start

```bash
# Build Hub Admin
docker build -t plugin-hub:go-latest .

# Import to k3s containerd
docker save plugin-hub:go-latest | sudo k3s ctr images import -

# Deploy
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
| GET | `/api/load-plugins` | Get `LOAD_PLUGINS` value (for ConfigMap sync) |

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
4. Dashboard: click **Enable**
5. Update `proxy-plugins` ConfigMap `LOAD_PLUGINS` with the new vendor (from `/api/load-plugins`)
6. Restart proxy pods → vendor goes live on ALL proxy instances

**Hub code never changes when adding vendors.**

## K8s Deployments

| Deployment | Replicas | Purpose |
|------------|:--------:|---------|
| hub-admin | 1 | Dashboard + API + reverse proxy |
| ticket-proxy | 2+ | All vendor plugins (scale for capacity) |

## ConfigMap

| ConfigMap | Key | Managed by | Used by |
|-----------|-----|:----------:|---------|
| proxy-plugins | LOAD_PLUGINS | Hub Admin (via `/api/load-plugins`) | ticket-proxy pods |

## Auto-Discovery

Hub fetches `plugin-manifest.json` from ticket-vendor's GitHub raw URL every 5 minutes.
Test results come from CI, not from the Hub.
