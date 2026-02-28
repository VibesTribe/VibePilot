#!/usr/bin/env python
"""
VibePilot Terminal Dashboard

Real-time view of VibePilot state:
- Active tasks
- Model status
- Runner pool
- Recent task runs

Usage:
    python dashboard/terminal_dashboard.py
    python dashboard/terminal_dashboard.py --watch (auto-refresh)
"""

import os
import sys
import json
import time
import argparse
from datetime import datetime
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from dotenv import load_dotenv
from supabase import create_client

load_dotenv()

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

if not SUPABASE_URL or not SUPABASE_KEY:
    print("ERROR: Missing SUPABASE_URL or SUPABASE_KEY")
    sys.exit(1)

db = create_client(SUPABASE_URL, SUPABASE_KEY)


def clear_screen():
    os.system("clear" if os.name == "posix" else "cls")


def format_duration(seconds):
    if seconds < 60:
        return f"{seconds}s"
    elif seconds < 3600:
        return f"{seconds // 60}m {seconds % 60}s"
    else:
        return f"{seconds // 3600}h {(seconds % 3600) // 60}m"


def format_timestamp(ts):
    if not ts:
        return "N/A"
    dt = datetime.fromisoformat(ts.replace("Z", "+00:00"))
    return dt.strftime("%H:%M:%S")


def get_task_counts():
    """Get task counts by status."""
    res = db.table("tasks").select("status").execute()
    tasks = res.data or []

    counts = {}
    for task in tasks:
        status = task.get("status", "unknown")
        counts[status] = counts.get(status, 0) + 1

    return counts


def get_active_tasks():
    """Get tasks that are in progress or available."""
    res = (
        db.table("tasks")
        .select("*")
        .in_("status", ["in_progress", "available", "assigned"])
        .execute()
    )
    return res.data or []


def get_recent_runs(limit=10):
    """Get recent task runs."""
    res = (
        db.table("task_runs")
        .select("*")
        .order("created_at", desc=True)
        .limit(limit)
        .execute()
    )
    return res.data or []


def get_model_status():
    """Get model status from config."""
    from core.config_loader import ConfigLoader

    config = ConfigLoader()
    return config.get_models()


def render_dashboard(watch=False):
    """Render the dashboard."""
    clear_screen()

    print("=" * 70)
    print(f"  VIBEPILOT DASHBOARD  |  {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 70)
    print()

    task_counts = get_task_counts()
    print("  TASK STATUS")
    print("  " + "-" * 40)
    status_order = [
        "available",
        "assigned",
        "in_progress",
        "review",
        "testing",
        "complete",
        "failed",
    ]
    for status in status_order:
        count = task_counts.get(status, 0)
        if count > 0:
            icon = {
                "complete": "✓",
                "failed": "✗",
                "in_progress": "►",
                "review": "?",
            }.get(status, "○")
            print(f"    {icon} {status}: {count}")
    print()

    active_tasks = get_active_tasks()
    if active_tasks:
        print("  ACTIVE TASKS")
        print("  " + "-" * 40)
        for task in active_tasks[:5]:
            task_id = task.get("id", "?")[:8]
            title = task.get("title", "Untitled")[:30]
            status = task.get("status", "?")
            assigned = (task.get("assigned_to") or "Unassigned")[:15]
            print(f"    {task_id}... | {status:12} | {assigned:15} | {title}")
        if len(active_tasks) > 5:
            print(f"    ... and {len(active_tasks) - 5} more")
        print()

    recent_runs = get_recent_runs(5)
    if recent_runs:
        print("  RECENT RUNS")
        print("  " + "-" * 40)
        for run in recent_runs:
            task_id = run.get("task_id", "?")[:8]
            model = run.get("model_id", "?")[:15]
            status = run.get("status", "?")
            duration = run.get("duration_seconds", 0) or 0
            tokens = run.get("tokens_total", 0) or 0
            icon = "✓" if status == "success" else "✗"
            print(
                f"    {icon} {task_id}... | {model:15} | {format_duration(duration):>6} | {tokens:>5} tokens"
            )
        print()

    models = get_model_status()
    print("  AVAILABLE MODELS")
    print("  " + "-" * 40)
    for model in models[:6]:
        model_id = model.get("id", "?")[:20]
        access = model.get("access_type", "?")[:15]
        context = model.get("context_limit", 0)
        if context >= 1000000:
            ctx_str = f"{context // 1000}k"
        else:
            ctx_str = f"{context // 1000}k"
        print(f"    ○ {model_id:20} | {access:15} | {ctx_str} ctx")
    if len(models) > 6:
        print(f"    ... and {len(models) - 6} more")
    print()

    print("=" * 70)
    if watch:
        print("  Press Ctrl+C to exit  |  Refreshing every 5s")
    else:
        print("  Run with --watch for auto-refresh")
    print("=" * 70)


def main():
    parser = argparse.ArgumentParser(description="VibePilot Terminal Dashboard")
    parser.add_argument(
        "--watch", "-w", action="store_true", help="Auto-refresh every 5 seconds"
    )
    args = parser.parse_args()

    if args.watch:
        try:
            while True:
                render_dashboard(watch=True)
                time.sleep(5)
        except KeyboardInterrupt:
            print("\nDashboard stopped.")
    else:
        render_dashboard(watch=False)


if __name__ == "__main__":
    main()
