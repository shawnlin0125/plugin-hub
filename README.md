# Plugin Hub

Universal plugin management platform.

## Architecture

```
plugin-hub (this repo)     ←  Platform: discovery + on/off + test gate
ticket-vendor (separate)   ←  Plugins: implements Plugin ABC
payment-vendor (future)    ←  Plugins: payment domain
```

## Quick Start

```bash
# Install platform
pip install -e .

# Run (no plugins loaded — just the API)
LOAD_PLUGINS="" python -m platform.main
```

## Adding a Plugin

1. Create a repo that implements `platform_plugin_sdk.Plugin`
2. Register via `entry_points` in pyproject.toml
3. Install it: `pip install git+https://github.com/org/your-plugin.git`
4. Set `LOAD_PLUGINS=your-plugin-id` and restart

## API

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/plugins | List all plugins |
| GET | /api/plugins/{id} | Plugin details |
| POST | /api/plugins/{id}/enable | Enable plugin (requires test pass) |
| POST | /api/plugins/{id}/disable | Disable plugin |
| POST | /api/plugins/{id}/test | Run isolated tests |
| GET | /api/plugins/{id}/health | Health check |
| GET | /health | Platform health |
