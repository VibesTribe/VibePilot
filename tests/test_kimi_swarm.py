#!/usr/bin/env python3
"""
Kimi Swarm Test (DEC-008)

Tests the Kimi swarm capability for parallel task execution.
"""

import os
import sys
import json
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from runners.kimi_runner import KimiRunner


def test_basic_availability():
    """Test 1: Kimi availability check."""
    print("\n" + "=" * 60)
    print("TEST 1: Kimi Availability")
    print("=" * 60)

    runner = KimiRunner()

    if runner.is_available():
        print("✅ Kimi is available")
        return True
    else:
        print("❌ Kimi is NOT available")
        print("   Ensure Kimi CLI is installed and in PATH")
        return False


def test_single_task():
    """Test 2: Single task execution."""
    print("\n" + "=" * 60)
    print("TEST 2: Single Task Execution")
    print("=" * 60)

    runner = KimiRunner()

    result = runner.execute_task("Return the word 'SUCCESS' and nothing else.")

    if result["success"] and "SUCCESS" in result.get("output", ""):
        print("✅ Single task executed successfully")
        print(f"   Output: {result['output'][:100]}")
        return True
    else:
        print(f"❌ Single task failed: {result.get('error', 'Unknown error')}")
        return False


def test_swarm_execution():
    """Test 3: Swarm parallel execution."""
    print("\n" + "=" * 60)
    print("TEST 3: Swarm Parallel Execution")
    print("=" * 60)

    runner = KimiRunner()

    tasks = [
        {"id": "task-1", "prompt": "Return 'ONE' and nothing else."},
        {"id": "task-2", "prompt": "Return 'TWO' and nothing else."},
        {"id": "task-3", "prompt": "Return 'THREE' and nothing else."},
    ]

    print(f"   Dispatching {len(tasks)} tasks to swarm...")
    start_time = time.time()

    result = runner.execute_swarm(tasks, max_workers=3)

    elapsed = time.time() - start_time

    print(f"   Elapsed: {elapsed:.2f}s")
    print(f"   Completed: {result['completed']}/{result['total']}")
    print(f"   Failed: {result['failed']}")

    if result["success"]:
        print("✅ Swarm execution successful")
        for r in result["results"]:
            print(f"   {r['task_id']}: {r.get('output', 'N/A')[:50]}")
        return True
    else:
        print("❌ Swarm execution had failures")
        for r in result["results"]:
            if not r.get("success"):
                print(f"   {r['task_id']}: {r.get('error', 'Unknown')}")
        return False


def test_swarm_scale():
    """Test 4: Larger scale swarm."""
    print("\n" + "=" * 60)
    print("TEST 4: Larger Scale Swarm (10 tasks)")
    print("=" * 60)

    runner = KimiRunner()

    tasks = [
        {"id": f"scale-{i}", "prompt": f"Return the number {i} and nothing else."}
        for i in range(1, 11)
    ]

    print(f"   Dispatching {len(tasks)} tasks...")
    start_time = time.time()

    result = runner.execute_swarm(tasks, max_workers=10)

    elapsed = time.time() - start_time

    print(f"   Elapsed: {elapsed:.2f}s")
    print(f"   Completed: {result['completed']}/{result['total']}")

    success_rate = result["completed"] / result["total"] * 100
    print(f"   Success rate: {success_rate:.0f}%")

    if success_rate >= 80:
        print("✅ Scale test passed (>=80% success)")
        return True
    else:
        print("❌ Scale test failed (<80% success)")
        return False


def test_orchestrator_integration():
    """Test 5: Orchestrator swarm detection."""
    print("\n" + "=" * 60)
    print("TEST 5: Orchestrator Integration")
    print("=" * 60)

    try:
        from core.orchestrator import ConcurrentOrchestrator

        orch = ConcurrentOrchestrator()

        regular_task = {"type": "feature", "prompt": "Build a feature"}
        audit_task = {"type": "repo_audit", "repo_path": os.getcwd()}
        multi_task = {
            "type": "feature",
            "subtasks": [
                {"prompt": "Task 1"},
                {"prompt": "Task 2"},
                {"prompt": "Task 3"},
            ],
        }

        regular_swarm = orch._should_use_swarm(regular_task)
        audit_swarm = orch._should_use_swarm(audit_task)
        multi_swarm = orch._should_use_swarm(multi_task)

        print(f"   Regular feature task → swarm: {regular_swarm} (expected: False)")
        print(f"   Repo audit task → swarm: {audit_swarm} (expected: True)")
        print(f"   Multi-subtask task → swarm: {multi_swarm} (expected: True)")

        if not regular_swarm and audit_swarm and multi_swarm:
            print("✅ Swarm detection working correctly")
            return True
        else:
            print("❌ Swarm detection not working as expected")
            return False

    except Exception as e:
        print(f"❌ Orchestrator integration failed: {e}")
        return False


def main():
    print("=" * 60)
    print("KIMI SWARM TEST (DEC-008)")
    print("=" * 60)

    results = []

    results.append(("Availability", test_basic_availability()))

    if not results[0][1]:
        print("\n⛔ Cannot continue - Kimi not available")
        return

    results.append(("Single Task", test_single_task()))
    results.append(("Swarm Execution", test_swarm_execution()))
    results.append(("Scale Test", test_swarm_scale()))
    results.append(("Orchestrator Integration", test_orchestrator_integration()))

    print("\n" + "=" * 60)
    print("SUMMARY")
    print("=" * 60)

    passed = sum(1 for _, r in results if r)
    total = len(results)

    for name, result in results:
        status = "✅ PASS" if result else "❌ FAIL"
        print(f"   {name}: {status}")

    print(f"\n   Total: {passed}/{total} tests passed")

    if passed == total:
        print("\n🎉 All tests passed! Kimi swarm is ready.")
    elif passed >= total * 0.6:
        print("\n⚠️  Most tests passed. Review failures above.")
    else:
        print("\n⛔ Major issues detected. Fix before using swarm.")


if __name__ == "__main__":
    main()
