#!/usr/bin/env python3
"""
VibePilot E2E Pipeline Test
Pushes a minimal PRD and tracks the full pipeline through to completion.

Usage:
  python3 scripts/e2e_pipeline_test.py [--wait-minutes 10]

Exits 0 if pipeline completes successfully, 1 on failure/timeout.
"""

import subprocess
import sys
import time
import json
import os
import argparse
from datetime import datetime, timezone

REPO_DIR = os.path.expanduser("~/VibePilot")
PRD_DIR = os.path.join(REPO_DIR, "docs/prd")
DB = "vibepilot"

def psql(query):
    """Run a psql query and return stripped output."""
    r = subprocess.run(
        ["psql", "-d", DB, "-t", "-A", "-c", query],
        capture_output=True, text=True, timeout=10
    )
    return r.stdout.strip()

def psql_json(query):
    """Run a psql query and return parsed JSON."""
    r = subprocess.run(
        ["psql", "-d", DB, "-t", "-A", "-c", query],
        capture_output=True, text=True, timeout=10
    )
    try:
        return json.loads(r.stdout.strip())
    except (json.JSONDecodeError, ValueError):
        return None

def create_test_prd():
    """Create a minimal test PRD file."""
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    filename = f"e2e-test-{timestamp}.md"
    filepath = os.path.join(PRD_DIR, filename)
    
    content = f"""# PRD: E2E Pipeline Test {timestamp}

Priority: Low
Complexity: Simple
Category: coding

## Context
Automated E2E pipeline verification test. This PRD should produce exactly one simple task.

## What to Build
- Create a file at `scripts/e2e-test-marker.txt` containing the current timestamp
- The file should have exactly one line: the ISO 8601 timestamp when it was created

## Files
- scripts/e2e-test-marker.txt - timestamp marker file

## Expected Output
- A single text file with an ISO 8601 timestamp

## Constraints
- Do NOT modify any existing files
- Do NOT create any files outside scripts/
- This is a test PRD — the output can be deleted after verification
"""
    
    with open(filepath, "w") as f:
        f.write(content)
    
    return filepath, filename

def git_commit_push(filepath, filename):
    """Commit and push the PRD to trigger the pipeline."""
    subprocess.run(["git", "add", filepath], cwd=REPO_DIR, check=True, capture_output=True)
    subprocess.run(
        ["git", "commit", "-m", f"prd: e2e pipeline test {filename}"],
        cwd=REPO_DIR, check=True, capture_output=True, timeout=30
    )
    subprocess.run(
        ["git", "push", "origin", "main"],
        cwd=REPO_DIR, check=True, capture_output=True, timeout=30
    )

def wait_for_plan(prd_filename, timeout_minutes=10):
    """Wait for a plan to be created from our PRD."""
    deadline = time.time() + timeout_minutes * 60
    while time.time() < deadline:
        # Check for any plans created recently
        result = psql_json(f"""
            SELECT id, status, plan_path 
            FROM plans 
            WHERE created_at > NOW() - INTERVAL '2 minutes'
            ORDER BY created_at DESC LIMIT 1
        """)
        if result:
            print(f"  Plan found: {result.get('id', '?')[:8]} status={result.get('status')}")
            return result
        time.sleep(5)
    return None

def wait_for_tasks(plan_id, timeout_minutes=15):
    """Wait for all tasks under a plan to reach a terminal state."""
    deadline = time.time() + timeout_minutes * 60
    terminal_statuses = {"complete", "merged", "human_review", "failed"}
    
    while time.time() < deadline:
        result = psql_json(f"""
            SELECT json_agg(json_build_object(
                'id', id, 'task_number', task_number, 'status', status,
                'attempts', attempts, 'assigned_to', assigned_to
            )) as tasks
            FROM tasks 
            WHERE plan_id = '{plan_id}'
        """)
        if result:
            tasks = result if isinstance(result, list) else []
            if not tasks:
                # Maybe no tasks yet
                time.sleep(5)
                continue
            
            all_terminal = all(t.get("status") in terminal_statuses for t in tasks)
            statuses = [f"{t.get('task_number','?')}={t.get('status')}" for t in tasks]
            print(f"  Tasks: {', '.join(statuses)}")
            
            if all_terminal:
                return tasks
        time.sleep(10)
    return None

def check_events(task_id=None, minutes=15):
    """Check pipeline events for our task/plan."""
    if task_id:
        result = psql(f"""
            SELECT event_type, model_id, reason, created_at
            FROM orchestrator_events
            WHERE task_id = '{task_id}'
            ORDER BY created_at ASC
        """)
    else:
        result = psql(f"""
            SELECT event_type, task_id, model_id, created_at
            FROM orchestrator_events
            WHERE created_at > NOW() - INTERVAL '{minutes} minutes'
            ORDER BY created_at ASC
        """)
    return result

def cleanup(plan_id=None, prd_path=None):
    """Clean up test data."""
    print("\nCleaning up test data...")
    if plan_id:
        subprocess.run(
            ["psql", "-d", DB, "-c", 
             f"DELETE FROM orchestrator_events WHERE task_id IN (SELECT id::text FROM tasks WHERE plan_id = '{plan_id}'); "
             f"DELETE FROM task_runs WHERE task_id IN (SELECT id FROM tasks WHERE plan_id = '{plan_id}'); "
             f"DELETE FROM tasks WHERE plan_id = '{plan_id}'; "
             f"DELETE FROM plans WHERE id = '{plan_id}';"],
            capture_output=True, timeout=10
        )
    if prd_path and os.path.exists(prd_path):
        os.remove(prd_path)
        subprocess.run(["git", "add", prd_path], cwd=REPO_DIR, capture_output=True)
        subprocess.run(
            ["git", "commit", "-m", "cleanup: remove e2e test PRD"],
            cwd=REPO_DIR, capture_output=True, timeout=30
        )
        subprocess.run(["git", "push", "origin", "main"], cwd=REPO_DIR, capture_output=True, timeout=30)

def main():
    parser = argparse.ArgumentParser(description="VibePilot E2E Pipeline Test")
    parser.add_argument("--wait-minutes", type=int, default=15, help="Max minutes to wait for completion")
    parser.add_argument("--no-cleanup", action="store_true", help="Don't clean up test data after")
    args = parser.parse_args()
    
    print("=" * 60)
    print("VibePilot E2E Pipeline Test")
    print(f"Started: {datetime.now().isoformat()}")
    print("=" * 60)
    
    # Pre-checks
    print("\n[1/6] Pre-checks...")
    gov_status = subprocess.run(
        ["systemctl", "--user", "is-active", "vibepilot-governor"],
        capture_output=True, text=True
    )
    if gov_status.stdout.strip() != "active":
        print(f"  FAIL: Governor is {gov_status.stdout.strip()}")
        return 1
    print(f"  Governor: active")
    
    db_check = psql("SELECT count(*) FROM tasks")
    print(f"  Database: connected ({db_check} existing tasks)")
    
    branch = subprocess.run(
        ["git", "branch", "--show-current"],
        cwd=REPO_DIR, capture_output=True, text=True
    )
    if branch.stdout.strip() != "main":
        print(f"  FAIL: Not on main branch (on {branch.stdout.strip()})")
        return 1
    print(f"  Branch: main")
    
    # Create PRD
    print("\n[2/6] Creating test PRD...")
    prd_path, prd_filename = create_test_prd()
    print(f"  Created: {prd_path}")
    
    # Push to GitHub
    print("\n[3/6] Pushing to GitHub...")
    try:
        git_commit_push(prd_path, prd_filename)
        print("  Pushed successfully")
    except subprocess.CalledProcessError as e:
        print(f"  FAIL: git push failed: {e.stderr if hasattr(e, 'stderr') else e}")
        return 1
    
    # Wait for plan
    print("\n[4/6] Waiting for plan creation...")
    plan = wait_for_plan(prd_filename, timeout_minutes=args.wait_minutes)
    if not plan:
        print("  FAIL: No plan created within timeout")
        events = check_events(minutes=args.wait_minutes)
        print(f"\n  Recent events:\n{events}")
        return 1
    
    plan_id = plan.get("id", "")
    print(f"  Plan ID: {plan_id[:8]}")
    
    # Wait for tasks
    print("\n[5/6] Waiting for task completion...")
    tasks = wait_for_tasks(plan_id, timeout_minutes=args.wait_minutes)
    if not tasks:
        print("  FAIL: Tasks did not complete within timeout")
        events = check_events(minutes=args.wait_minutes)
        print(f"\n  Recent events:\n{events}")
        return 1
    
    # Analyze results
    print("\n[6/6] Analyzing results...")
    success = True
    for t in tasks:
        status = t.get("status")
        tid = t.get("id", "?")[:8]
        tnum = t.get("task_number", "?")
        attempts = t.get("attempts", 0)
        model = t.get("assigned_to", "?")
        
        if status in ("merged", "complete"):
            print(f"  PASS: Task {tnum} → {status} (model={model}, attempts={attempts})")
        elif status == "human_review":
            print(f"  DIAGNOSTIC: Task {tnum} → human_review (hit diagnostic ceiling, attempts={attempts})")
            success = False
        else:
            print(f"  FAIL: Task {tnum} → {status} (attempts={attempts})")
            success = False
    
    # Show pipeline events
    print("\n  Pipeline events:")
    for t in tasks:
        events = check_events(task_id=t.get("id", ""))
        if events:
            for line in events.split("\n"):
                if line.strip():
                    print(f"    {line.strip()}")
    
    # Cleanup
    if not args.no_cleanup:
        cleanup(plan_id=plan_id, prd_path=prd_path)
    
    print("\n" + "=" * 60)
    if success:
        print("E2E TEST: PASSED")
        print("=" * 60)
        return 0
    else:
        print("E2E TEST: FAILED")
        print("=" * 60)
        return 1

if __name__ == "__main__":
    sys.exit(main())
