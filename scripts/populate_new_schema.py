#!/usr/bin/env python3
"""
VibePilot Data Migration Script

Populates new tables (models_new, tools, access) from config files.

Run AFTER creating tables with migrations/001_data_model_redesign.sql

Usage:
    python scripts/populate_new_schema.py
"""

import json
import sys
from pathlib import Path
from datetime import datetime

sys.path.insert(0, str(Path(__file__).parent.parent))

from dotenv import load_dotenv

load_dotenv()

from supabase import create_client
import os


def get_supabase():
    url = os.getenv("SUPABASE_URL")
    key = os.getenv("SUPABASE_KEY")
    if not url or not key:
        raise ValueError("SUPABASE_URL and SUPABASE_KEY must be set")
    return create_client(url, key)


def load_config(filename: str) -> dict:
    config_path = Path(__file__).parent.parent / "config" / filename
    with open(config_path) as f:
        return json.load(f)


def populate_models(db):
    """Populate models_new from config/models.json"""
    print("\n=== POPULATING models_new ===")

    config = load_config("models.json")
    models = config.get("models", [])

    populated = 0
    for model in models:
        model_id = model.get("id")

        row = {
            "id": model_id,
            "name": model.get("name", model_id),
            "provider": model.get("provider", "unknown"),
            "capabilities": model.get("capabilities", []),
            "context_limit": model.get("context_limit"),
            "cost_input_per_1k_usd": model.get("cost_input_per_1k_usd"),
            "cost_output_per_1k_usd": model.get("cost_output_per_1k_usd"),
            "notes": model.get("notes"),
        }

        try:
            result = db.table("models_new").upsert(row).execute()
            if result.data:
                populated += 1
                print(f"  ✓ {model_id}")
        except Exception as e:
            print(f"  ✗ {model_id}: {e}")

    print(f"Populated {populated}/{len(models)} models")
    return populated


def populate_tools(db):
    """Populate tools table"""
    print("\n=== POPULATING tools ===")

    # Tools are interfaces, not models
    tools = [
        {
            "id": "opencode",
            "name": "OpenCode CLI",
            "type": "cli",
            "supported_providers": ["zhipu"],  # Currently GLM only
            "has_codebase_access": True,
            "has_browser_control": False,
            "runner_class": None,  # Direct CLI, not via runner
        },
        {
            "id": "kimi-cli",
            "name": "Kimi CLI",
            "type": "cli",
            "supported_providers": ["moonshot"],
            "has_codebase_access": True,
            "has_browser_control": True,  # Kimi has browser use
            "runner_class": "KimiContractRunner",
        },
        {
            "id": "direct-api",
            "name": "Direct API",
            "type": "api",
            "supported_providers": ["deepseek", "google", "anthropic", "openai"],
            "has_codebase_access": False,
            "has_browser_control": False,
            "runner_class": None,  # Dynamically selected
        },
        {
            "id": "courier",
            "name": "Courier (Browser)",
            "type": "courier",
            "supported_providers": ["all"],  # Can go to any web platform
            "has_codebase_access": False,
            "has_browser_control": True,
            "runner_class": "CourierContractRunner",
        },
    ]

    populated = 0
    for tool in tools:
        try:
            result = db.table("tools").upsert(tool).execute()
            if result.data:
                populated += 1
                print(f"  ✓ {tool['id']}")
        except Exception as e:
            print(f"  ✗ {tool['id']}: {e}")

    print(f"Populated {populated}/{len(tools)} tools")
    return populated


def populate_access(db):
    """Populate access table linking models to tools"""
    print("\n=== POPULATING access ===")

    config = load_config("models.json")
    models = config.get("models", [])

    # Get existing statuses from old models table
    old_models = {}
    try:
        result = (
            db.table("models")
            .select(
                "id, status, status_reason, cooldown_expires_at, subscription_ends_at"
            )
            .execute()
        )
        for m in result.data or []:
            old_models[m["id"]] = m
    except Exception as e:
        print(f"  Warning: Could not get old model statuses: {e}")

    # Gemini rate limits from research
    gemini_limits = {
        "gemini-2.5-flash": {"rpm": 10, "rpd": 250, "tpm": 250000},
        "gemini-2.5-flash-lite": {"rpm": 15, "rpd": 1000, "tpm": 250000},
        "gemini-1.5-flash": {"rpm": 15, "rpd": 1500, "tpm": 1000000},
        "gemini-2.0-flash": {"rpm": 10, "rpd": 500, "tpm": 250000},
    }

    access_records = []

    for model in models:
        model_id = model.get("id")
        access_via = model.get("access_via", [])

        for dest in access_via:
            # Determine tool and method based on destination
            if dest == "opencode":
                tool_id = "opencode"
                method = "subscription"
                priority = 0
                platform_id = None
            elif dest == "kimi":
                tool_id = "kimi-cli"
                method = "subscription"
                priority = 0
                platform_id = None
            elif dest == "deepseek-api":
                tool_id = "direct-api"
                method = "api"
                priority = 2
                platform_id = None
            elif dest == "gemini-api":
                tool_id = "direct-api"
                method = "api"
                priority = 2
                platform_id = None
            elif dest in [
                "chatgpt-web",
                "claude-web",
                "gemini-web",
                "copilot-web",
                "deepseek-web",
            ]:
                tool_id = "courier"
                method = "web_free_tier"
                priority = 1
                # Map to platform ID
                platform_map = {
                    "chatgpt-web": "chatgpt",
                    "claude-web": "claude",
                    "gemini-web": "gemini",
                    "copilot-web": "copilot",
                    "deepseek-web": "deepseek",
                }
                platform_id = platform_map.get(dest)
            else:
                print(f"  Skipping unknown destination: {dest} for {model_id}")
                continue

            # Get status from old table
            old_info = old_models.get(model_id, {})
            status = old_info.get("status", "active")
            status_reason = old_info.get("status_reason")
            cooldown_until = old_info.get("cooldown_expires_at")

            # Special handling for known situations
            if model_id in ["gemini-api", "gemini-2.0-flash", "gemini-2.5-flash"]:
                status = "paused"
                status_reason = "quota_exhausted"
            elif model_id == "deepseek-chat" and dest == "deepseek-api":
                status = "paused"
                status_reason = "credit_needed"
            elif model_id in [
                "gpt-4o",
                "gpt-4o-mini",
                "claude-sonnet-4-5",
                "claude-haiku-4-5",
            ]:
                # Web only, no API - these should be accessible via courier
                status = "active"
                status_reason = None

            # Get rate limits
            limits = gemini_limits.get(model_id, {})

            record = {
                "model_id": model_id,
                "tool_id": tool_id,
                "platform_id": platform_id,
                "method": method,
                "priority": priority,
                "status": status,
                "status_reason": status_reason,
                "cooldown_until": cooldown_until,
                "requests_per_minute": limits.get("rpm"),
                "requests_per_day": limits.get("rpd"),
                "tokens_per_minute": limits.get("tpm"),
            }

            # Subscription info
            if method == "subscription":
                if dest == "kimi":
                    record["subscription_cost_usd"] = 0.99  # Current promo
                    record["subscription_ends_at"] = "2026-02-27T00:00:00Z"

            access_records.append(record)

    populated = 0
    for record in access_records:
        try:
            result = db.table("access").insert(record).execute()
            if result.data:
                populated += 1
                print(
                    f"  ✓ {record['model_id']} via {record['tool_id']} ({record['method']})"
                )
        except Exception as e:
            err = str(e)
            if "duplicate" not in err.lower():
                print(f"  ✗ {record['model_id']}: {err[:80]}")

    print(f"Populated {populated}/{len(access_records)} access records")
    return populated


def verify(db):
    """Verify the migration"""
    print("\n=== VERIFICATION ===")

    tables = ["models_new", "tools", "access"]
    for table in tables:
        try:
            result = db.table(table).select("*", count="exact").execute()
            count = result.count if hasattr(result, "count") else len(result.data or [])
            print(f"  {table}: {count} rows")
        except Exception as e:
            print(f"  {table}: ERROR - {e}")

    # Check specific relationships
    print("\n=== SAMPLE ACCESS RECORDS ===")
    try:
        result = (
            db.table("access").select("*").eq("status", "active").limit(5).execute()
        )
        for r in result.data or []:
            print(
                f"  {r['model_id']} via {r['tool_id']} | {r['method']} | priority={r['priority']}"
            )
    except Exception as e:
        print(f"  Error: {e}")


def main():
    print("=" * 60)
    print("VIBEPILOT DATA MODEL MIGRATION")
    print("=" * 60)
    print("\nMake sure you've run migrations/001_data_model_redesign.sql first!")

    db = get_supabase()

    # Check tables exist
    for table in ["models_new", "tools", "access"]:
        try:
            db.table(table).select("*").limit(0).execute()
            print(f"  ✓ {table} exists")
        except Exception as e:
            print(f"  ✗ {table} does not exist - run SQL migration first!")
            return 1

    # Populate
    populate_models(db)
    populate_tools(db)
    populate_access(db)

    # Verify
    verify(db)

    print("\n" + "=" * 60)
    print("MIGRATION COMPLETE")
    print("=" * 60)
    print("\nNext steps:")
    print("1. Update orchestrator to use new tables")
    print("2. Test routing with new schema")
    print("3. Once verified, rename old 'models' table to 'models_backup'")
    print("4. Rename 'models_new' to 'models'")

    return 0


if __name__ == "__main__":
    sys.exit(main())
