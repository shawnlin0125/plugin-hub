"""Plugin ABC — the one contract every plugin must fulfill.

This module defines the universal interface that ALL plugin-hub plugins
MUST implement, regardless of domain (tickets, payments, logistics, ...).

Design principles:
  - Zero business logic — no mention of "ticket", "payment", etc.
  - Self-contained — plugins carry their own mock, fixtures, schema.
  - Testable in isolation — the test gate runs against mock only.
"""

from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from typing import Any


# ── SDK Data Models ──────────────────────────────────────────────

@dataclass
class HealthStatus:
    """Plugin health check result."""
    healthy: bool
    latency_ms: float
    message: str = ""


@dataclass
class TestResult:
    """Result of a single test case."""
    name: str
    passed: bool
    duration_ms: float
    error: str | None = None


@dataclass
class TestReport:
    """Aggregated test report for one plugin."""
    plugin_id: str
    plugin_version: str
    passed: bool
    total: int = 0
    passed_count: int = 0
    results: list[TestResult] = field(default_factory=list)

    @property
    def summary(self) -> str:
        return f"{self.plugin_id} v{self.plugin_version}: {self.passed_count}/{self.total} passed"


@dataclass
class Fixture:
    """A named test fixture (JSON data file)."""
    name: str
    description: str
    data: dict[str, Any]


# ── Plugin ABC ───────────────────────────────────────────────────

class Plugin(ABC):
    """Universal plugin contract.

    Every plugin for plugin-hub:
      1. Implements this ABC.
      2. Declares itself via setuptools entry_points under 'plugin_hub.plugins'.
      3. Carries its own mock server and fixtures for isolated testing.

    The platform calls:
      - discover()   → find all installed plugins
      - test()       → run isolated tests (against mock only)
      - start()      → activate the plugin (opens external connections)
      - stop()       → deactivate the plugin
      - health()     → check if the plugin is running correctly
    """

    # ── Metadata (override in subclass) ──

    plugin_id: str          # e.g. "ticketmaster"
    plugin_name: str        # e.g. "TicketMaster"
    version: str = "0.1.0"

    # ── Lifecycle ──

    @abstractmethod
    async def start(self) -> None:
        """Start the plugin.

        Called by the platform when the plugin is enabled.
        The plugin should:
          - Open external API connections
          - Start its own scheduler (APScheduler, cron, ...)
          - Register health check
        """
        ...

    @abstractmethod
    async def stop(self) -> None:
        """Stop the plugin.

        Called by the platform when the plugin is disabled.
        The plugin should:
          - Close external connections gracefully
          - Stop its scheduler
          - Flush pending work
        """
        ...

    @abstractmethod
    async def health(self) -> HealthStatus:
        """Check plugin health.

        Called periodically by the platform for monitoring.
        Should return quickly (< 5s).
        """
        ...

    # ── Testing Support (required by the platform's test gate) ──

    @abstractmethod
    def get_mock_server(self) -> Any:
        """Return a mock server instance for this plugin.

        The mock server:
          - Simulates the external API
          - Runs on 127.0.0.1:<random_port>
          - Serves data from the plugin's fixtures/

        The platform uses this to run tests in complete isolation
        — no real external API calls during testing.
        """
        ...

    @abstractmethod
    def get_fixtures(self) -> list[Fixture]:
        """Return all test fixtures for this plugin.

        Fixtures are the fake data used by mock and tests.
        They live in the plugin's own fixtures/ directory.
        """
        ...

    @abstractmethod
    async def run_tests(self, mock_port: int) -> TestReport:
        """Run all plugin tests against the mock server.

        Called by the platform's TestGate.
        The plugin runs its own test suite, pointing at
        http://127.0.0.1:{mock_port} instead of real APIs.

        Returns a TestReport — if report.passed is True,
        the plugin can be enabled.
        """
        ...

    # ── Schema (DB isolation) ──

    @property
    def db_schema(self) -> str:
        """PostgreSQL schema name for this plugin.

        Default: plugin_{plugin_id}
        Override if you need a custom schema name.
        """
        return f"plugin_{self.plugin_id}"

    @property
    def redis_prefix(self) -> str:
        """Redis key prefix for this plugin.

        Default: plugin:{plugin_id}:
        All keys used by this plugin MUST start with this prefix.
        """
        return f"plugin:{self.plugin_id}:"
