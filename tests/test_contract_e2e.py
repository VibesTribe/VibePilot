#!/usr/bin/env python
"""
End-to-End Test for VibePilot Contract Layer

Tests the full flow:
1. Load config
2. Create a test task packet
3. Execute via contract runner
4. Verify result format

Usage:
    python tests/test_contract_e2e.py
"""

import os
import sys
import json
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from core.config_loader import ConfigLoader
from runners.contract_runners import get_runner, RUNNER_REGISTRY


def test_config_loader():
    """Test config loading."""
    print("\n=== Testing Config Loader ===\n")

    config = ConfigLoader()
    result = config.validate_configs()

    assert result["valid"], f"Config validation failed: {result['errors']}"
    print(f"✓ Config valid")
    print(f"  - Skills: {result['stats']['skills']}")
    print(f"  - Tools: {result['stats']['tools']}")
    print(f"  - Models: {result['stats']['models']}")
    print(f"  - Agents: {result['stats']['agents']}")
    print(f"  - Platforms: {result['stats']['platforms']}")

    planner = config.get_agent_with_prompt("planner")
    assert planner, "Planner agent not found"
    assert planner.get("model") == "kimi-cli", f"Wrong model: {planner.get('model')}"
    assert len(planner.get("resolved_skills", [])) > 0, "No skills resolved"
    print(f"✓ Planner agent loaded correctly")

    return True


def test_runner_probes():
    """Test runner health checks."""
    print("\n=== Testing Runner Probes ===\n")

    runners_to_test = ["kimi", "gemini", "deepseek"]

    for runner_id in runners_to_test:
        try:
            runner = get_runner(runner_id)
            success, message = runner.probe()
            status = "✓" if success else "✗"
            print(f"  {status} {runner_id}: {message}")
        except Exception as e:
            print(f"  ✗ {runner_id}: Error - {e}")

    return True


def test_contract_runner_execute():
    """Test contract runner execution."""
    print("\n=== Testing Contract Runner Execution ===\n")

    task_packet = {
        "task_id": "test-e2e-001",
        "title": "Test Task",
        "prompt": "Respond with exactly: 'VibePilot test successful'",
        "objectives": ["Return test message"],
        "deliverables": [],
        "context": "End-to-end test",
        "output_format": {"type": "text"},
        "constraints": {"max_tokens": 50, "timeout_seconds": 60},
        "runner_context": {"attempt_number": 1, "previous_failures": []},
    }

    runner = get_runner("kimi")
    result = runner.execute(task_packet)

    print(f"  Task ID: {result.get('task_id')}")
    print(f"  Status: {result.get('status')}")
    print(f"  Output: {result.get('output', '')[:100]}")
    print(f"  Duration: {result.get('metadata', {}).get('duration_seconds')}s")
    print(
        f"  Tokens: {result.get('metadata', {}).get('tokens_in', 0)} in, {result.get('metadata', {}).get('tokens_out', 0)} out"
    )

    assert result.get("task_id") == "test-e2e-001", "Wrong task_id"
    assert result.get("status") in ["success", "failed"], (
        f"Invalid status: {result.get('status')}"
    )
    assert "metadata" in result, "Missing metadata"
    assert "feedback" in result, "Missing feedback"

    if result.get("status") == "success":
        assert result.get("output"), "No output on success"
        print(f"  ✓ Task completed successfully")
    else:
        print(f"  ! Task failed (expected if runner unavailable)")
        errors = result.get("errors", [])
        if errors:
            print(f"    Error: {errors[0].get('message', 'Unknown')}")

    return True


def test_result_schema():
    """Test result matches schema."""
    print("\n=== Testing Result Schema Compliance ===\n")

    runner = get_runner("kimi")

    task_packet = {
        "task_id": "test-schema-001",
        "prompt": "Say 'test'",
        "constraints": {"max_tokens": 10},
    }

    result = runner.execute(task_packet)

    required_fields = [
        "task_id",
        "status",
        "output",
        "artifacts",
        "errors",
        "metadata",
        "feedback",
    ]
    for field in required_fields:
        assert field in result, f"Missing required field: {field}"
        print(f"  ✓ {field}: present")

    metadata = result.get("metadata", {})
    metadata_fields = [
        "model",
        "platform",
        "tokens_in",
        "tokens_out",
        "duration_seconds",
    ]
    for field in metadata_fields:
        assert field in metadata, f"Missing metadata field: {field}"
    print(f"  ✓ All metadata fields present")

    feedback = result.get("feedback", {})
    feedback_fields = ["output_matched_prompt", "suggested_next_step", "virtual_cost"]
    for field in feedback_fields:
        assert field in feedback, f"Missing feedback field: {field}"
    print(f"  ✓ All feedback fields present")

    return True


def test_invalid_input():
    """Test handling of invalid input."""
    print("\n=== Testing Invalid Input Handling ===\n")

    runner = get_runner("kimi")

    invalid_packet = {"title": "Missing task_id and prompt"}

    is_valid, error = runner.validate_input(invalid_packet)
    assert not is_valid, "Should have failed validation"
    assert error, "Should have error message"
    print(f"  ✓ Invalid input rejected: {error}")

    return True


def main():
    """Run all tests."""
    print("=" * 60)
    print("VIBEPILOT CONTRACT LAYER E2E TEST")
    print("=" * 60)

    tests = [
        ("Config Loader", test_config_loader),
        ("Runner Probes", test_runner_probes),
        ("Contract Runner Execute", test_contract_runner_execute),
        ("Result Schema", test_result_schema),
        ("Invalid Input", test_invalid_input),
    ]

    passed = 0
    failed = 0

    for name, test_func in tests:
        try:
            if test_func():
                passed += 1
            else:
                failed += 1
        except Exception as e:
            print(f"\n  ✗ {name} failed with exception: {e}")
            failed += 1

    print("\n" + "=" * 60)
    print(f"RESULTS: {passed} passed, {failed} failed")
    print("=" * 60)

    return failed == 0


if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)
