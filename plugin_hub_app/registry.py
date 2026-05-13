"""Plugin Registry — manages plugin lifecycle: load, enable, disable, status.

The registry is the single source of truth for which plugins exist
and whether they are currently running.

State machine:
    unloaded → loaded (discovered, installed)
    loaded   → enabled (test passed + admin toggled on)
    enabled  → disabled (admin toggled off)
"""

from dataclasses import dataclass, field
from enum import Enum
from platform_plugin_sdk import Plugin, TestReport


class PluginState(Enum):
    UNLOADED = "unloaded"    # Not discovered yet
    LOADED = "loaded"        # Discovered but not enabled
    ENABLED = "enabled"      # Running
    DISABLED = "disabled"    # Was running, now stopped
    ERROR = "error"          # Failed to start


@dataclass
class PluginEntry:
    """A single plugin in the registry."""
    plugin_id: str
    plugin_cls: type[Plugin]
    instance: Plugin | None = None
    state: PluginState = PluginState.UNLOADED
    test_report: TestReport | None = None
    route_target: str = ""   # e.g. "vendor-heavy-1"


class PluginRegistry:
    """Central registry for all plugins."""

    def __init__(self):
        self._plugins: dict[str, PluginEntry] = {}

    # ── CRUD ──

    def register(self, plugin_cls: type[Plugin], route_target: str = "") -> None:
        """Register a discovered plugin class."""
        # Instantiate to get metadata
        inst = plugin_cls()
        entry = PluginEntry(
            plugin_id=inst.plugin_id,
            plugin_cls=plugin_cls,
            instance=inst,
            state=PluginState.LOADED,
            route_target=route_target,
        )
        self._plugins[inst.plugin_id] = entry

    async def enable(self, plugin_id: str) -> None:
        """Enable a plugin (start it)."""
        entry = self._plugins[plugin_id]
        if entry.state == PluginState.ENABLED:
            return
        if entry.test_report is None or not entry.test_report.passed:
            raise CannotEnable(f"Plugin '{plugin_id}' has not passed tests")

        await entry.instance.start()
        entry.state = PluginState.ENABLED

    async def disable(self, plugin_id: str) -> None:
        """Disable a plugin (stop it gracefully)."""
        entry = self._plugins[plugin_id]
        if entry.state != PluginState.ENABLED:
            return
        await entry.instance.stop()
        entry.state = PluginState.DISABLED

    def record_test(self, plugin_id: str, report: TestReport) -> None:
        """Record a test result for a plugin."""
        self._plugins[plugin_id].test_report = report

    # ── Queries ──

    def get(self, plugin_id: str) -> PluginEntry | None:
        return self._plugins.get(plugin_id)

    def list_all(self) -> list[dict]:
        """Return summary of all plugins for API/UI."""
        return [
            {
                "plugin_id": e.plugin_id,
                "plugin_name": e.instance.plugin_name,
                "version": e.instance.version,
                "state": e.state.value,
                "test_passed": e.test_report.passed if e.test_report else False,
                "route_target": e.route_target,
            }
            for e in self._plugins.values()
        ]

    def list_enabled(self) -> list[PluginEntry]:
        return [e for e in self._plugins.values() if e.state == PluginState.ENABLED]


class CannotEnable(Exception):
    """Raised when a plugin cannot be enabled (e.g. tests not passed)."""
