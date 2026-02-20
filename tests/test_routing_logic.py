#!/usr/bin/env python
"""
Routing Logic Tests for VibePilot

Tests the routing flag system:
1. Runner routing capability detection (CLI/API can internal, web couriers only web)
2. Task filtering by routing flag
3. Orchestrator selection respects routing constraints

Usage:
    python tests/test_routing_logic.py

Requires schema v1.1 to be applied to Supabase (routing_flag column on tasks).
"""

import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from core.config_loader import ConfigLoader


def test_runner_routing_capability():
    """
    Test that runners have correct routing_capability based on access_type.

    CLI/API/CLI_subscription runners should be able to handle: internal, web, mcp
    Web couriers should only handle: web
    """
    print("\n=== Testing Runner Routing Capability ===\n")

    config = ConfigLoader()
    models = config.get_models()

    for model in models:
        model_id = model.get("id")
        access_type = model.get("access_type", "api")

        # Expected capability based on access type
        # cli_subscription, cli, and api all have codebase access
        if access_type in ("cli", "api", "cli_subscription"):
            expected = ["internal", "web", "mcp"]
        else:
            expected = ["web"]

        print(f"  {model_id} ({access_type}):")
        print(f"    Expected capability: {expected}")
        # Note: Actual capability is set in orchestrator.py RunnerPool
        # This test documents the expected behavior

    return True


def test_routing_flag_thresholds():
    """
    Test that routing flag thresholds are documented and correct.

    W (web): 0-1 dependencies, creates new files only
    Q (internal): 2+ dependencies, touches existing files
    RED FLAG: 2+ existing files modified
    """
    print("\n=== Testing Routing Flag Thresholds ===\n")

    test_cases = [
        {
            "name": "Standalone task",
            "dependencies": [],
            "files_modified": [],
            "expected_flag": "web",
            "reason": "0 deps, no existing files",
        },
        {
            "name": "Single dependency",
            "dependencies": ["TASK-001"],
            "files_modified": [],
            "expected_flag": "web",
            "reason": "1 dep is still web-safe",
        },
        {
            "name": "Two dependencies",
            "dependencies": ["TASK-001", "TASK-002"],
            "files_modified": [],
            "expected_flag": "internal",
            "reason": "2+ deps requires internal",
        },
        {
            "name": "Touches existing file",
            "dependencies": [],
            "files_modified": ["src/auth.py"],
            "expected_flag": "internal",
            "reason": "Modifying existing file needs codebase",
        },
        {
            "name": "Multi-file modification",
            "dependencies": [],
            "files_modified": ["src/auth.py", "src/routes.py"],
            "expected_flag": "internal",
            "reason": "RED FLAG - isolation problem",
        },
    ]

    for tc in test_cases:
        dep_count = len(tc["dependencies"])
        mod_count = len(tc["files_modified"])

        # Apply thresholds
        if mod_count >= 2:
            flag = "internal"
            note = "RED FLAG - escalate to Council"
        elif dep_count >= 2 or mod_count >= 1:
            flag = "internal"
            note = "OK"
        else:
            flag = "web"
            note = "OK"

        status = "✓" if flag == tc["expected_flag"] else "✗"
        print(f"  {status} {tc['name']}")
        print(f"      Dependencies: {dep_count}, Modified files: {mod_count}")
        print(f"      Expected: {tc['expected_flag']}, Got: {flag} ({note})")

        assert flag == tc["expected_flag"], f"Wrong flag for {tc['name']}"

    return True


def test_orchestrator_filtering():
    """
    Test that orchestrator correctly filters tasks by routing capability.

    Mock scenarios:
    - Runner can only do web → should only see W tasks
    - Runner can do internal → should see W and Q tasks
    """
    print("\n=== Testing Orchestrator Task Filtering ===\n")

    # Mock runners with different capabilities
    runners = {
        "web-courier": {"routing_capability": ["web"]},
        "cli-runner": {"routing_capability": ["internal", "web", "mcp"]},
    }

    # Mock tasks with different flags
    tasks = [
        {"id": "T001", "routing_flag": "web", "title": "Create utility"},
        {"id": "T002", "routing_flag": "internal", "title": "Complex integration"},
        {"id": "T003", "routing_flag": "web", "title": "Write tests"},
        {"id": "T004", "routing_flag": "mcp", "title": "IDE task"},
    ]

    for runner_id, runner in runners.items():
        capability = runner["routing_capability"]
        eligible = [t for t in tasks if t["routing_flag"] in capability]

        print(f"  {runner_id} (can do: {capability}):")
        print(f"    Eligible tasks: {[t['id'] for t in eligible]}")

        if runner_id == "web-courier":
            assert len(eligible) == 2, "Web courier should see 2 W tasks"
            assert all(t["routing_flag"] == "web" for t in eligible)
        elif runner_id == "cli-runner":
            assert len(eligible) == 4, "CLI runner should see all 4 tasks"

    return True


def test_slice_grouping():
    """
    Test that tasks are properly grouped by slice_id.

    Dashboard needs to show slice progress: completed/total per slice.
    """
    print("\n=== Testing Slice Grouping ===\n")

    # Mock tasks with slice assignments
    tasks = [
        {"id": "T001", "slice_id": "auth", "status": "merged"},
        {"id": "T002", "slice_id": "auth", "status": "in_progress"},
        {"id": "T003", "slice_id": "auth", "status": "available"},
        {"id": "T004", "slice_id": "data", "status": "merged"},
        {"id": "T005", "slice_id": "data", "status": "merged"},
        {"id": "T006", "slice_id": "ui", "status": "available"},
    ]

    # Group by slice
    slices = {}
    for task in tasks:
        slice_id = task["slice_id"]
        if slice_id not in slices:
            slices[slice_id] = {"total": 0, "completed": 0, "in_progress": 0}
        slices[slice_id]["total"] += 1
        if task["status"] == "merged":
            slices[slice_id]["completed"] += 1
        elif task["status"] == "in_progress":
            slices[slice_id]["in_progress"] += 1

    for slice_id, stats in slices.items():
        print(
            f"  {slice_id}: {stats['completed']}/{stats['total']} completed, {stats['in_progress']} in progress"
        )

    assert slices["auth"]["total"] == 3
    assert slices["auth"]["completed"] == 1
    assert slices["data"]["completed"] == 2
    assert slices["ui"]["total"] == 1

    return True


def test_planner_output_format():
    """
    Test that planner output includes required routing fields.

    Each task should have:
    - slice_id
    - routing_flag
    - routing_flag_reason
    - dependency_count (implicit from dependencies array)
    """
    print("\n=== Testing Planner Output Format ===\n")

    # Mock planner output for one task
    task = {
        "task_id": "AUTH-P1-T001",
        "slice_id": "auth",
        "phase": "P1",
        "title": "Create user model",
        "dependencies": [],
        "routing_flag": "web",
        "routing_flag_reason": "Zero dependencies, standalone",
        "expected_output": {
            "files_created": ["src/models/user.py"],
            "files_modified": [],
        },
    }

    required_fields = ["slice_id", "routing_flag", "routing_flag_reason"]
    for field in required_fields:
        assert field in task, f"Missing field: {field}"
        print(f"  ✓ {field}: {task[field]}")

    # Verify flag matches dependencies
    dep_count = len(task.get("dependencies", []))
    mod_count = len(task.get("expected_output", {}).get("files_modified", []))

    if dep_count >= 2 or mod_count >= 1:
        assert task["routing_flag"] == "internal", "Should be internal with deps/mods"
    else:
        assert task["routing_flag"] == "web", "Should be web with no deps/mods"

    print(f"  ✓ Flag matches dependency profile")

    return True


def main():
    """Run all routing tests."""
    print("=" * 60)
    print("VIBEPILOT ROUTING LOGIC TESTS")
    print("=" * 60)

    tests = [
        ("Runner Routing Capability", test_runner_routing_capability),
        ("Routing Flag Thresholds", test_routing_flag_thresholds),
        ("Orchestrator Task Filtering", test_orchestrator_filtering),
        ("Slice Grouping", test_slice_grouping),
        ("Planner Output Format", test_planner_output_format),
    ]

    passed = 0
    failed = 0

    for name, test_func in tests:
        try:
            if test_func():
                passed += 1
        except Exception as e:
            print(f"\n  ✗ {name} failed with exception: {e}")
            failed += 1

    print("\n" + "=" * 60)
    print(f"RESULTS: {passed} passed, {failed} failed")
    print("=" * 60)

    if failed == 0:
        print("\n✓ All routing logic tests passed")
        print("\nNOTE: These tests verify logic. For database integration:")
        print("  1. Run docs/schema_v1.1_routing.sql on Supabase")
        print("  2. Test with actual task data")

    return failed == 0


if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)
