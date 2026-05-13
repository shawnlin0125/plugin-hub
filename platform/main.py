"""Platform API Server — FastAPI entry point.

plugin-hub is the universal plugin management platform.
It handles:
  - Plugin discovery (via setuptools entry_points)
  - Plugin on/off toggling
  - Test gate (isolated testing before enable)
  - Vendor routing (maps plugins → k8s deployments)
  - Health monitoring

The platform has NO business logic — that lives in plugins.
"""

import os
import asyncio
from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException
from platform.discovery import discover_plugins
from platform.registry import PluginRegistry, CannotEnable
from platform.router import VendorRouter
from platform.test_gate import gate_check


# ── Globals ──────────────────────────────────────────────────────

registry = PluginRegistry()
router = VendorRouter()


# ── Lifecycle ─────────────────────────────────────────────────────

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup: discover plugins. Shutdown: stop all."""
    # Startup
    print("\n═══ Plugin Hub Starting ═══")
    print(f"   LOAD_PLUGINS={os.environ.get('LOAD_PLUGINS', 'all')}")

    discovered = discover_plugins()
    allowed = os.environ.get("LOAD_PLUGINS", "")

    for plugin_id, plugin_cls in discovered.items():
        # Filter by LOAD_PLUGINS if set
        if allowed and allowed != "all" and plugin_id not in allowed.split(","):
            print(f"   ⏭  Skipping {plugin_id} (not in LOAD_PLUGINS)")
            continue

        target = router.resolve(plugin_id)
        registry.register(plugin_cls, route_target=target)

    print(f"   Registered {len(registry.list_all())} plugins\n")

    yield  # App runs here

    # Shutdown
    print("\n═══ Plugin Hub Shutting Down ═══")
    for entry in registry.list_enabled():
        print(f"   Stopping {entry.plugin_id}...")
        await entry.instance.stop()
    print("   All plugins stopped\n")


app = FastAPI(
    title="Plugin Hub",
    description="Universal plugin management platform",
    version="0.1.0",
    lifespan=lifespan,
)


# ── API Endpoints ─────────────────────────────────────────────────

@app.get("/api/plugins")
async def list_plugins():
    """List all discovered plugins and their status."""
    return registry.list_all()


@app.get("/api/plugins/{plugin_id}")
async def get_plugin(plugin_id: str):
    """Get details for a single plugin."""
    entry = registry.get(plugin_id)
    if not entry:
        raise HTTPException(404, f"Plugin '{plugin_id}' not found")
    return {
        "plugin_id": entry.plugin_id,
        "plugin_name": entry.instance.plugin_name,
        "version": entry.instance.version,
        "state": entry.state.value,
        "test_passed": entry.test_report.passed if entry.test_report else False,
        "route_target": entry.route_target,
    }


@app.post("/api/plugins/{plugin_id}/enable")
async def enable_plugin(plugin_id: str):
    """Enable a plugin. Requires test to have passed."""
    entry = registry.get(plugin_id)
    if not entry:
        raise HTTPException(404, f"Plugin '{plugin_id}' not found")
    try:
        await registry.enable(plugin_id)
        return {"status": "enabled", "plugin_id": plugin_id}
    except CannotEnable as e:
        raise HTTPException(403, str(e))


@app.post("/api/plugins/{plugin_id}/disable")
async def disable_plugin(plugin_id: str):
    """Disable a running plugin."""
    entry = registry.get(plugin_id)
    if not entry:
        raise HTTPException(404, f"Plugin '{plugin_id}' not found")
    await registry.disable(plugin_id)
    return {"status": "disabled", "plugin_id": plugin_id}


@app.post("/api/plugins/{plugin_id}/test")
async def test_plugin(plugin_id: str):
    """Run isolated tests for a plugin."""
    entry = registry.get(plugin_id)
    if not entry:
        raise HTTPException(404, f"Plugin '{plugin_id}' not found")

    passed, report = await gate_check(entry.instance)
    registry.record_test(plugin_id, report)

    return {
        "plugin_id": plugin_id,
        "passed": passed,
        "summary": report.summary,
        "results": [
            {"name": r.name, "passed": r.passed, "duration_ms": r.duration_ms}
            for r in report.results
        ],
    }


@app.get("/api/plugins/{plugin_id}/health")
async def health_plugin(plugin_id: str):
    """Check plugin health."""
    entry = registry.get(plugin_id)
    if not entry:
        raise HTTPException(404, f"Plugin '{plugin_id}' not found")
    if entry.state.value != "enabled":
        raise HTTPException(400, f"Plugin '{plugin_id}' is not running")

    status = await entry.instance.health()
    return {
        "plugin_id": plugin_id,
        "healthy": status.healthy,
        "latency_ms": status.latency_ms,
        "message": status.message,
    }


@app.get("/api/routing")
async def get_routing():
    """Return the current routing table."""
    return {
        "plugins": {
            pid: router.resolve(pid)
            for pid in [e["plugin_id"] for e in registry.list_all()]
        },
        "deployments": list(router.get_deployments()),
    }


@app.get("/health")
async def platform_health():
    """Platform-level health check."""
    plugins = registry.list_all()
    enabled = sum(1 for p in plugins if p["state"] == "enabled")
    return {
        "status": "healthy",
        "plugins_total": len(plugins),
        "plugins_enabled": enabled,
    }


# ── Direct entry point ───────────────────────────────────────────

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
