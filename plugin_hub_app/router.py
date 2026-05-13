"""Vendor Router — maps plugin_id → k8s deployment target.

Reads from a ConfigMap:
    vendor-routing:
      ticketmaster: {target: vendor-heavy-1}
      kktix:        {target: vendor-heavy-2}
      ibon:         {target: vendor-bundle}
      fami:         {target: vendor-bundle}
"""

import yaml
from pathlib import Path


class VendorRouter:
    """Routes plugins to deployment targets based on ConfigMap."""

    def __init__(self, routing_config_path: str = "/etc/plugin-hub/routing.yaml"):
        self.routing: dict[str, dict] = {}
        self._load(routing_config_path)

    def _load(self, path: str) -> None:
        cfg = Path(path)
        if cfg.exists():
            data = yaml.safe_load(cfg.read_text()) or {}
            self.routing = data
            print(f"✓  Loaded routing config: {len(data)} entries")
        else:
            print(f"⚠  No routing config at {path}, using default (vendor-bundle)")

    def resolve(self, plugin_id: str) -> str:
        """Return the deployment target name.

        e.g. resolve("ticketmaster") → "vendor-heavy-1"
             resolve("unknown")     → "vendor-bundle" (default)
        """
        entry = self.routing.get(plugin_id, {})
        return entry.get("target", "vendor-bundle")

    def get_deployments(self) -> set[str]:
        """Return all unique deployment targets."""
        return {e.get("target", "vendor-bundle") for e in self.routing.values()}
