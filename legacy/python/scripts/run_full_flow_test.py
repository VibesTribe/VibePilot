#!/usr/bin/env python3
"""
Test script for VibePilot full flow.

Usage:
    python scripts/test_full_flow.py

This tests the complete flow:
1. process_idea() - Creates PRD and Plan in GitHub
2. review_and_approve_plan() - Council reviews and creates tasks
3. (Manual) Watch orchestrator pick up and execute tasks
"""

import os
import sys
import json

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from dotenv import load_dotenv

load_dotenv()

from core.orchestrator import ConcurrentOrchestrator


def test_entry_layer():
    """Test process_idea() creates PRD and Plan in GitHub."""
    print("\n" + "=" * 60)
    print("TEST 1: process_idea() - Entry Layer")
    print("=" * 60)

    orchestrator = ConcurrentOrchestrator()

    idea = "Add a simple hello world function to test the system"
    project_id = "test-project-001"

    print(f"\nInput idea: {idea}")
    print(f"Project ID: {project_id}")

    result = orchestrator.process_idea(idea, project_id, save_to_github=True)

    print("\n--- RESULT ---")
    print(f"Success: {result.get('success')}")

    if result.get("success"):
        print(f"PRD path: {result.get('prd_path')}")
        print(f"Plan path: {result.get('plan_path')}")
        print(f"Task count: {result.get('task_count')}")
        print(f"Tasks: {json.dumps(result.get('tasks', [])[:2], indent=2)[:500]}...")
    else:
        print(f"Error: {result.get('error')}")
        print(f"Details: {result.get('details')}")

    return result


def test_council_and_task_creation(plan_path: str, project_id: str):
    """Test Council review and task creation."""
    print("\n" + "=" * 60)
    print("TEST 2: review_and_approve_plan() - Council + Tasks")
    print("=" * 60)

    orchestrator = ConcurrentOrchestrator()

    print(f"\nPlan path: {plan_path}")
    print(f"Project ID: {project_id}")

    result = orchestrator.review_and_approve_plan(plan_path, project_id)

    print("\n--- RESULT ---")
    print(f"Success: {result.get('success')}")
    print(f"Approved: {result.get('approved')}")

    if result.get("approved"):
        print(f"Tasks created: {result.get('tasks_created')}")
        print(f"Task IDs: {result.get('task_ids')}")
    else:
        print("Council did NOT approve the plan")
        print(f"Council result: {result.get('council_result')}")

    return result


def main():
    print("=" * 60)
    print("VIBEPILOT FULL FLOW TEST")
    print("=" * 60)

    step1_result = test_entry_layer()

    if step1_result.get("success"):
        plan_path = step1_result.get("plan_path")
        project_id = "test-project-001"

        print("\n" + "-" * 60)
        input("Press Enter to continue to Council review...")

        step2_result = test_council_and_task_creation(plan_path, project_id)

        if step2_result.get("approved"):
            print("\n" + "=" * 60)
            print("SUCCESS! Tasks created in Supabase.")
            print("=" * 60)
            print("\nNext steps:")
            print("1. Check Supabase for tasks with status 'pending'")
            print("2. Orchestrator should pick them up via _tick()")
            print("3. Watch task execution in orchestrator logs:")
            print("   journalctl -u vibepilot-orchestrator -f")
    else:
        print("\nEntry layer test FAILED")
        sys.exit(1)


if __name__ == "__main__":
    main()
