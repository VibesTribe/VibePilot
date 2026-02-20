#!/usr/bin/env python
"""
Test: Create and dispatch a task through VibePilot pipeline

Usage:
    python tests/test_pipeline.py
"""

import os
import sys
import json
import uuid
from datetime import datetime
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from dotenv import load_dotenv
from supabase import create_client

load_dotenv()

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

db = create_client(SUPABASE_URL, SUPABASE_KEY)


def create_test_task():
    """Create a test task in Supabase."""
    task_id = str(uuid.uuid4())

    task = {
        "id": task_id,
        "title": "Test Task: Generate a greeting",
        "type": "test",
        "status": "available",
        "priority": 5,
        "dependencies": [],
        "created_at": datetime.utcnow().isoformat(),
        "updated_at": datetime.utcnow().isoformat(),
    }

    res = db.table("tasks").insert(task).execute()

    if res.data:
        print(f"✓ Created task: {task_id[:8]}...")

        prompt_packet = {
            "task_id": task_id,
            "title": "Generate a greeting",
            "prompt": "Write a friendly greeting message. Be creative but keep it under 50 words.",
            "objectives": ["Create a friendly greeting"],
            "deliverables": ["text response"],
            "context": "Testing the VibePilot pipeline",
            "output_format": {"type": "text"},
            "constraints": {"max_tokens": 100, "timeout_seconds": 60},
        }

        packet = {
            "task_id": task_id,
            "prompt": json.dumps(prompt_packet),
            "tech_spec": json.dumps({"type": "test", "context": "Pipeline test"}),
            "version": 1,
        }
        db.table("task_packets").insert(packet).execute()

        return task_id
    else:
        print(f"✗ Failed to create task: {res}")
        return None


def dispatch_task(task_id):
    """Dispatch task using contract runner."""
    from runners.contract_runners import get_runner

    res = db.table("task_packets").select("*").eq("task_id", task_id).execute()
    if not res.data:
        print(f"✗ Task packet not found: {task_id}")
        return None

    packet = res.data[0]
    prompt_packet = json.loads(packet.get("prompt", "{}"))

    print(f"\n→ Dispatching task to Kimi runner...")

    runner = get_runner("kimi")
    result = runner.execute(prompt_packet)

    print(f"\n← Result:")
    print(f"  Status: {result.get('status')}")
    output = result.get("output") or ""
    print(f"  Output: {output[:200] if output else 'N/A'}")
    print(f"  Duration: {result.get('metadata', {}).get('duration_seconds')}s")
    print(
        f"  Tokens: {result.get('metadata', {}).get('tokens_in')} in, {result.get('metadata', {}).get('tokens_out')} out"
    )

    task_run = {
        "task_id": task_id,
        "model_id": "kimi-k2.5",
        "platform": "cli",
        "courier": "internal",
        "status": result.get("status"),
        "result": result.get("output"),
        "tokens_used": result.get("metadata", {}).get("tokens_in", 0)
        + result.get("metadata", {}).get("tokens_out", 0),
    }

    db.table("task_runs").insert(task_run).execute()

    if result.get("status") == "success":
        db.table("tasks").update(
            {
                "status": "merged",
                "result": result.get("output"),
                "completed_at": datetime.utcnow().isoformat(),
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()
        print(f"✓ Task marked merged")
    else:
        db.table("tasks").update(
            {
                "status": "failed",
                "result": result.get("errors", [{}])[0].get("message", "Unknown error"),
                "updated_at": datetime.utcnow().isoformat(),
            }
        ).eq("id", task_id).execute()
        print(f"✗ Task marked failed")

    return result


def main():
    print("=" * 60)
    print("VIBEPILOT PIPELINE TEST")
    print("=" * 60)

    print("\n1. Creating test task...")
    task_id = create_test_task()

    if not task_id:
        print("Failed to create task")
        return 1

    print("\n2. Dispatching task...")
    result = dispatch_task(task_id)

    if result and result.get("status") == "success":
        print("\n" + "=" * 60)
        print("✓ PIPELINE TEST PASSED")
        print("=" * 60)
        return 0
    else:
        print("\n" + "=" * 60)
        print("✗ PIPELINE TEST FAILED")
        print("=" * 60)
        return 1


if __name__ == "__main__":
    sys.exit(main())
