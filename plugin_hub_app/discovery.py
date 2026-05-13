"""Plugin Discovery — find all installed plugins via setuptools entry_points.

Plugins register themselves in pyproject.toml:

    [project.entry-points."plugin_hub.plugins"]
    ticketmaster = "vendor_ticketmaster.plugin:TicketmasterPlugin"
    kktix = "vendor_kktix.plugin:KKTIXPlugin"

The discovery module scans all installed packages for these entry points
and returns the loaded Plugin classes.
"""

from importlib.metadata import entry_points
from platform_plugin_sdk import Plugin


def discover_plugins() -> dict[str, type[Plugin]]:
    """Scan entry_points group 'plugin_hub.plugins' and return {id: PluginClass}.

    Returns:
        dict mapping plugin_id → Plugin subclass (not instantiated yet)

    Example:
        {
            "ticketmaster": TicketmasterPlugin,
            "kktix": KKTIxPlugin,
        }
    """
    plugins: dict[str, type[Plugin]] = {}

    try:
        eps = entry_points(group="plugin_hub.plugins")
    except TypeError:
        # Python < 3.12 fallback
        eps = entry_points().get("plugin_hub.plugins", [])

    for ep in eps:
        try:
            plugin_cls = ep.load()
            if not issubclass(plugin_cls, Plugin):
                print(f"⚠  {ep.name}: {plugin_cls} does not implement Plugin ABC, skipping")
                continue
            plugins[ep.name] = plugin_cls
            print(f"✓  Discovered plugin: {ep.name} → {plugin_cls.__module__}")
        except Exception as e:
            print(f"✗  Failed to load plugin '{ep.name}': {e}")

    return plugins
