#!/usr/bin/env python3
"""
Config Sync Script (Bidirectional)

Syncs between VibePilot config files and Supabase.

Import (JSON → Supabase): Seed, recovery, initial setup
Export (Supabase → JSON): Backup, portability

Usage:
    python scripts/sync_config_to_supabase.py          # Import: JSON → Supabase
    python scripts/sync_config_to_supabase.py --export # Export: Supabase → JSON
"""

import json
import sys
import argparse
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


def load_json_config(filename: str) -> dict:
    config_path = Path(__file__).parent.parent / "config" / filename
    with open(config_path) as f:
        return json.load(f)


def save_json_config(filename: str, data: dict):
    config_path = Path(__file__).parent.parent / "config" / filename
    with open(config_path, "w") as f:
        json.dump(data, f, indent=2)
    print(f"  Saved: {config_path}")


def import_models(db):
    """Import models.json to Supabase."""
    print("Importing models.json → Supabase...")

    config = load_json_config("models.json")
    models = config.get("models", [])

    synced = 0
    for model in models:
        model_id = model.get("id")

        row = {
            "id": model_id,
            "name": model.get("name", model_id),
            "vendor": model.get("provider", "Unknown"),
            "platform": model.get("provider", model_id),
            "courier": model.get("access_type", "api"),
            "access_type": model.get("access_type", "api"),
            "context_limit": model.get("context_limit", 32000),
            "status": model.get("status", "active"),
            "config": model,  # Store full config as JSONB
            "subscription_status": model.get("subscription_status"),
            "subscription_ends": model.get("subscription_ends"),
            "subscription_cost": model.get("subscription_cost"),
            "request_limit": model.get("request_limit"),
            "token_limit": model.get("token_limit"),
            "updated_at": datetime.utcnow().isoformat(),
        }

        result = db.table("models").upsert(row).execute()
        if result.data:
            synced += 1
            print(f"  ✓ {model_id}")

    print(f"Imported {synced}/{len(models)} models")
    return synced


def import_platforms(db):
    """Import platforms.json to Supabase."""
    print("Importing platforms.json → Supabase...")

    config = load_json_config("platforms.json")
    platforms = config.get("platforms", [])

    synced = 0
    for platform in platforms:
        platform_id = platform.get("id")

        row = {
            "id": platform_id,
            "name": platform.get("name", platform_id),
            "vendor": platform.get("provider", "Unknown"),
            "type": platform.get("type", "web"),
            "url": platform.get("url", ""),
            "context_limit": platform.get("free_tier", {}).get("context_limit"),
            "status": platform.get("status", "active"),
            "config": platform,  # Store full config as JSONB
            "updated_at": datetime.utcnow().isoformat(),
        }

        result = db.table("platforms").upsert(row).execute()
        if result.data:
            synced += 1
            print(f"  ✓ {platform_id}")

    print(f"Imported {synced}/{len(platforms)} platforms")
    return synced


def import_skills(db):
    """Import skills.json to Supabase."""
    print("Importing skills.json → Supabase...")

    config = load_json_config("skills.json")
    skills = config.get("skills", [])

    synced = 0
    for skill in skills:
        row = {
            "id": skill.get("id"),
            "name": skill.get("name", skill.get("id")),
            "description": skill.get("description"),
            "config": skill,
            "status": skill.get("status", "active"),
            "updated_at": datetime.utcnow().isoformat(),
        }

        result = db.table("skills").upsert(row).execute()
        if result.data:
            synced += 1
            print(f"  ✓ {skill.get('id')}")

    print(f"Imported {synced}/{len(skills)} skills")
    return synced


def import_tools(db):
    """Import tools.json to Supabase."""
    print("Importing tools.json → Supabase...")

    config = load_json_config("tools.json")
    tools = config.get("tools", [])

    synced = 0
    for tool in tools:
        row = {
            "id": tool.get("id"),
            "name": tool.get("name", tool.get("id")),
            "description": tool.get("description"),
            "config": tool,
            "status": tool.get("status", "active"),
            "updated_at": datetime.utcnow().isoformat(),
        }

        result = db.table("tools").upsert(row).execute()
        if result.data:
            synced += 1
            print(f"  ✓ {tool.get('id')}")

    print(f"Imported {synced}/{len(tools)} tools")
    return synced


def import_prompts(db):
    """Import prompts/*.md to Supabase."""
    print("Importing prompts/*.md → Supabase...")

    prompts_dir = Path(__file__).parent.parent / "config" / "prompts"

    synced = 0
    for prompt_file in prompts_dir.glob("*.md"):
        agent_id = prompt_file.stem
        content = prompt_file.read_text()

        row = {
            "id": agent_id,
            "agent_id": agent_id,
            "content": content,
            "status": "active",
            "updated_at": datetime.utcnow().isoformat(),
        }

        result = db.table("prompts").upsert(row).execute()
        if result.data:
            synced += 1
            print(f"  ✓ {agent_id}")

    print(f"Imported {synced} prompts")
    return synced


def export_models(db):
    """Export Supabase models to models.json."""
    print("Exporting Supabase → models.json...")

    result = db.table("models").select("*").eq("status", "active").execute()

    models = []
    for row in result.data or []:
        # Use stored config if available, otherwise build from columns
        if row.get("config"):
            models.append(row["config"])
        else:
            models.append(
                {
                    "id": row["id"],
                    "name": row.get("name"),
                    "provider": row.get("vendor"),
                    "access_type": row.get("access_type"),
                    "context_limit": row.get("context_limit"),
                    "status": row.get("status"),
                }
            )

    config = {
        "version": "1.0",
        "exported_at": datetime.utcnow().isoformat(),
        "models": models,
    }

    save_json_config("models.json", config)
    print(f"Exported {len(models)} models")
    return len(models)


def export_platforms(db):
    """Export Supabase platforms to platforms.json."""
    print("Exporting Supabase → platforms.json...")

    result = db.table("platforms").select("*").eq("status", "active").execute()

    platforms = []
    for row in result.data or []:
        if row.get("config"):
            platforms.append(row["config"])
        else:
            platforms.append(
                {
                    "id": row["id"],
                    "name": row.get("name"),
                    "url": row.get("url"),
                    "type": row.get("type"),
                    "status": row.get("status"),
                }
            )

    config = {
        "version": "1.0",
        "exported_at": datetime.utcnow().isoformat(),
        "platforms": platforms,
    }

    save_json_config("platforms.json", config)
    print(f"Exported {len(platforms)} platforms")
    return len(platforms)


def export_skills(db):
    """Export Supabase skills to skills.json."""
    print("Exporting Supabase → skills.json...")

    result = db.table("skills").select("*").eq("status", "active").execute()

    skills = []
    for row in result.data or []:
        if row.get("config"):
            skills.append(row["config"])
        else:
            skills.append(
                {
                    "id": row["id"],
                    "name": row.get("name"),
                    "description": row.get("description"),
                }
            )

    config = {
        "version": "1.0",
        "exported_at": datetime.utcnow().isoformat(),
        "skills": skills,
    }

    save_json_config("skills.json", config)
    print(f"Exported {len(skills)} skills")
    return len(skills)


def export_tools(db):
    """Export Supabase tools to tools.json."""
    print("Exporting Supabase → tools.json...")

    result = db.table("tools").select("*").eq("status", "active").execute()

    tools = []
    for row in result.data or []:
        if row.get("config"):
            tools.append(row["config"])
        else:
            tools.append(
                {
                    "id": row["id"],
                    "name": row.get("name"),
                    "description": row.get("description"),
                }
            )

    config = {
        "version": "1.0",
        "exported_at": datetime.utcnow().isoformat(),
        "tools": tools,
    }

    save_json_config("tools.json", config)
    print(f"Exported {len(tools)} tools")
    return len(tools)


def export_prompts(db):
    """Export Supabase prompts to prompts/*.md."""
    print("Exporting Supabase → prompts/*.md...")

    prompts_dir = Path(__file__).parent.parent / "config" / "prompts"
    prompts_dir.mkdir(exist_ok=True)

    result = db.table("prompts").select("*").eq("status", "active").execute()

    exported = 0
    for row in result.data or []:
        prompt_file = prompts_dir / f"{row['agent_id']}.md"
        prompt_file.write_text(row["content"])
        print(f"  ✓ {row['agent_id']}")
        exported += 1

    print(f"Exported {exported} prompts")
    return exported


def do_import(db):
    """Import all config from JSON to Supabase."""
    print("=" * 50)
    print("IMPORT: JSON → Supabase")
    print("=" * 50)
    print()

    import_models(db)
    import_platforms(db)

    # Try to import skills/tools/prompts if tables exist
    try:
        import_skills(db)
    except Exception as e:
        print(f"  Skills skipped: {e}")

    try:
        import_tools(db)
    except Exception as e:
        print(f"  Tools skipped: {e}")

    try:
        import_prompts(db)
    except Exception as e:
        print(f"  Prompts skipped: {e}")

    print()
    print("=" * 50)
    print("IMPORT COMPLETE")
    print("=" * 50)


def do_export(db):
    """Export all config from Supabase to JSON."""
    print("=" * 50)
    print("EXPORT: Supabase → JSON")
    print("=" * 50)
    print()

    export_models(db)
    export_platforms(db)

    try:
        export_skills(db)
    except Exception as e:
        print(f"  Skills skipped: {e}")

    try:
        export_tools(db)
    except Exception as e:
        print(f"  Tools skipped: {e}")

    try:
        export_prompts(db)
    except Exception as e:
        print(f"  Prompts skipped: {e}")

    print()
    print("=" * 50)
    print("EXPORT COMPLETE")
    print("=" * 50)


def main():
    parser = argparse.ArgumentParser(
        description="Sync config between JSON and Supabase"
    )
    parser.add_argument(
        "--export", action="store_true", help="Export from Supabase to JSON"
    )
    args = parser.parse_args()

    db = get_supabase()

    if args.export:
        do_export(db)
    else:
        do_import(db)


if __name__ == "__main__":
    main()
