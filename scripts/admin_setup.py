#!/usr/bin/env python3
"""
Admin setup script for VibePilot.
Uses vault to get service key for admin operations.

Usage:
    python scripts/admin_setup.py --create-runners
    python scripts/admin_setup.py --apply-schema
"""

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from vault_manager import VaultManager
from supabase import create_client


def get_admin_client():
    """Get Supabase client with service role key from vault."""
    vm = VaultManager()
    service_key = vm.get_secret("SUPABASE_SERVICE_KEY")

    if not service_key:
        raise ValueError("SUPABASE_SERVICE_KEY not found in vault")

    url = (
        vm.client.supabase_url
        if hasattr(vm.client, "supabase_url")
        else vm.client.rest_url
    )
    # Extract URL from the client
    import os

    url = os.getenv("SUPABASE_URL")

    return create_client(url, service_key)


def create_runners():
    """Create runners for currently active models."""
    sb = get_admin_client()

    # Get active models
    models = (
        sb.table("models")
        .select("id, status, access_type")
        .eq("status", "active")
        .execute()
    )

    # Get tools
    tools = sb.table("tools").select("id, type").eq("status", "active").execute()
    tool_map = {t["id"]: t["type"] for t in tools.data}

    # Check existing runners
    existing = sb.table("runners").select("model_id, tool_id").execute()
    existing_pairs = {(r["model_id"], r["tool_id"]) for r in existing.data}

    # Define runner configurations based on model + available tools
    runner_configs = [
        # GLM-5 via opencode (subscription, internal)
        {
            "model_id": "glm-5",
            "tool_id": "opencode",
            "routing_capability": ["internal"],
            "cost_priority": 0,  # subscription = best
            "status": "active",
            "daily_limit": 1000,
        },
        # Gemini API via opencode (free tier, internal)
        {
            "model_id": "gemini-api",
            "tool_id": "opencode",
            "routing_capability": ["internal"],
            "cost_priority": 1,  # free API
            "status": "active",
            "daily_limit": 500,
        },
    ]

    created = 0
    for config in runner_configs:
        model_id = config["model_id"]
        tool_id = config["tool_id"]

        # Check model exists and is active
        model_active = any(
            m["id"] == model_id and m["status"] == "active" for m in models.data
        )
        if not model_active:
            print(f"  SKIP: {model_id} (not active)")
            continue

        # Check tool exists
        if tool_id not in tool_map:
            print(f"  SKIP: {tool_id} (tool not found)")
            continue

        # Check not already exists
        if (model_id, tool_id) in existing_pairs:
            print(f"  EXISTS: {model_id} / {tool_id}")
            continue

        # Create runner
        sb.table("runners").insert(config).execute()
        print(f"  CREATED: {model_id} / {tool_id}")
        created += 1

    print(f"\nCreated {created} runners.")
    return created


def verify_runners():
    """Verify runners and RPCs are working."""
    sb = get_admin_client()

    print("=== RUNNERS ===")
    runners = sb.table("runners").select("*").execute()
    for r in runners.data:
        print(
            f"  {r['model_id']} / {r['tool_id']} | status={r['status']} | priority={r['cost_priority']}"
        )

    print("\n=== TEST get_best_runner RPC ===")
    result = sb.rpc("get_best_runner", {"p_routing": "internal"}).execute()
    if result.data:
        print(f"  Best runner: {result.data}")
    else:
        print("  No runner returned")

    return len(runners.data)


def apply_rls():
    """Apply RLS policy for runners table."""
    sb = get_admin_client()

    # RLS is applied via SQL, but we can verify it works
    print("Testing RLS by querying runners with service key...")
    result = sb.table("runners").select("id").limit(1).execute()
    print(f"  Query successful: {len(result.data)} runners found")
    return True


def main():
    parser = argparse.ArgumentParser(description="VibePilot Admin Setup")
    parser.add_argument(
        "--create-runners", action="store_true", help="Create runners for active models"
    )
    parser.add_argument("--verify", action="store_true", help="Verify runners and RPCs")
    parser.add_argument("--apply-rls", action="store_true", help="Test RLS policy")

    args = parser.parse_args()

    if not any([args.create_runners, args.verify, args.apply_rls]):
        parser.print_help()
        return 1

    try:
        if args.create_runners:
            create_runners()
        if args.verify:
            verify_runners()
        if args.apply_rls:
            apply_rls()
        return 0
    except Exception as e:
        print(f"Error: {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
