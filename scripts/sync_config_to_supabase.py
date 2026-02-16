#!/usr/bin/env python3
"""
Config Sync Script

Syncs VibePilot config files to Supabase for dashboard access.
Run this whenever models.json or platforms.json changes.

Usage:
    python scripts/sync_config_to_supabase.py
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


def get_logo_url(name: str, vendor: str = None) -> str:
    """Get logo URL based on name/vendor."""
    name_lower = name.lower()
    vendor_lower = (vendor or "").lower()

    # Map to lobehub icons
    icon_map = {
        "openai": "openai",
        "google": "google-gemini",
        "gemini": "google-gemini",
        "anthropic": "anthropic",
        "claude": "anthropic",
        "deepseek": "deepseek",
        "mistral": "mistral",
        "moonshot": "default",
        "kimi": "default",
        "zhipu": "default",
        "cursor": "cursor",
        "copilot": "github",
        "huggingface": "huggingface",
        "huggingchat": "huggingface",
    }

    for key, icon in icon_map.items():
        if key in name_lower or key in vendor_lower:
            return f"https://raw.githubusercontent.com/lobehub/lobe-icons/main/icons/{icon}.svg"

    return "https://raw.githubusercontent.com/lobehub/lobe-icons/main/icons/default.svg"


def sync_models(db):
    """Sync models.json to Supabase models table."""
    print("Syncing models.json...")

    config = load_config("models.json")
    models = config.get("models", [])

    synced = 0
    for model in models:
        model_id = model.get("id")

        # Prepare row data
        row = {
            "id": model_id,
            "name": model.get("name", model_id),
            "vendor": model.get("provider", "Unknown"),
            "platform": model.get("provider", model_id),
            "courier": model.get("access_type", "api"),
            "access_type": model.get("access_type", "api"),
            "context_limit": model.get("context_limit", 32000),
            "logo_url": get_logo_url(model.get("name", ""), model.get("provider")),
            "status": model.get("status", "active"),
            "subscription_status": model.get("subscription_status"),
            "subscription_ends": model.get("subscription_ends"),
            "subscription_cost": model.get("subscription_cost"),
            "request_limit": model.get("request_limit"),
            "token_limit": model.get("token_limit"),
            "updated_at": datetime.utcnow().isoformat(),
        }

        # Upsert
        result = db.table("models").upsert(row).execute()
        if result.data:
            synced += 1
            print(f"  ✓ {model_id}")
        else:
            print(f"  ✗ {model_id}: {result.error}")

    print(f"Synced {synced}/{len(models)} models")
    return synced


def sync_platforms(db):
    """Sync platforms.json to Supabase platforms table."""
    print("Syncing platforms.json...")

    config = load_config("platforms.json")
    platforms = config.get("platforms", [])

    synced = 0
    for platform in platforms:
        platform_id = platform.get("id")

        # Prepare row data
        row = {
            "id": platform_id,
            "name": platform.get("name", platform_id),
            "vendor": platform.get("vendor", "Unknown"),
            "type": platform.get("type", "web"),
            "context_limit": platform.get("context_limit", 32000),
            "request_limit": platform.get("request_limit"),
            "logo_url": get_logo_url(platform.get("name", ""), platform.get("vendor")),
            "status": platform.get("status", "active"),
            "updated_at": datetime.utcnow().isoformat(),
        }

        # Upsert
        result = db.table("platforms").upsert(row).execute()
        if result.data:
            synced += 1
            print(f"  ✓ {platform_id}")
        else:
            print(f"  ✗ {platform_id}: {result.error}")

    print(f"Synced {synced}/{len(platforms)} platforms")
    return synced


def verify(db):
    """Verify sync results."""
    print("\nVerifying...")

    models = db.table("models").select("id, name, status").execute()
    platforms = db.table("platforms").select("id, name, status").execute()

    print(f"  Models in DB: {len(models.data or [])}")
    for m in models.data or []:
        print(f"    - {m['id']}: {m['name']} ({m['status']})")

    print(f"  Platforms in DB: {len(platforms.data or [])}")
    for p in platforms.data or []:
        print(f"    - {p['id']}: {p['name']} ({p['status']})")


def main():
    print("=" * 50)
    print("VIBESPILOT CONFIG SYNC")
    print("=" * 50)
    print()

    db = get_supabase()

    models_synced = sync_models(db)
    platforms_synced = sync_platforms(db)

    verify(db)

    print()
    print("=" * 50)
    print(f"COMPLETE: {models_synced} models, {platforms_synced} platforms")
    print("=" * 50)


if __name__ == "__main__":
    main()
