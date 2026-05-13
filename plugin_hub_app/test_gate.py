"""Test Gate — the gatekeeper.

Before a plugin can be enabled, it must pass the test gate.
The test gate:
  1. Starts the plugin's mock server on a random port
  2. Tells the plugin to run its own test suite against the mock
  3. Records the result
  4. Cleans up (stops mock, drops test schema)

Only plugins with TestReport.passed == True can be enabled.
"""

import asyncio
from platform_plugin_sdk import Plugin, TestReport


class TestGate:
    """Runs isolated tests for a plugin."""

    async def run(self, plugin: Plugin) -> TestReport:
        """Run the full test suite for a plugin in isolation.

        1. Start mock server
        2. Run plugin's self-test
        3. Clean up
        """
        mock = plugin.get_mock_server()

        try:
            # 1. Start mock on random port
            mock_port = await mock.start(port=0)
            print(f"   Mock server for '{plugin.plugin_id}' listening on :{mock_port}")

            # 2. Run plugin's own tests
            report = await plugin.run_tests(mock_port=mock_port)
            print(f"   {plugin.plugin_id}: {report.summary}")

            return report

        finally:
            # 3. Always clean up
            await mock.stop()
            print(f"   Mock server for '{plugin.plugin_id}' stopped")


async def gate_check(plugin: Plugin) -> tuple[bool, TestReport]:
    """Convenience: run test gate and return (passed, report)."""
    gate = TestGate()
    report = await gate.run(plugin)
    return report.passed, report
