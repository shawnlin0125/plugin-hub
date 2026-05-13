"""Platform Plugin SDK — the universal plugin contract.

All plugins for plugin-hub MUST implement Plugin from this module.
No business logic here — only the abstract interface.

Usage:
    from platform_plugin_sdk import Plugin

    class MyVendorPlugin(Plugin):
        ...
"""

from .abc import Plugin, HealthStatus, TestReport, TestResult, Fixture

__all__ = ["Plugin", "HealthStatus", "TestReport", "TestResult", "Fixture"]
__version__ = "0.1.0"
