#!/usr/bin/env python3
"""
Update Hermes config fallback_providers from live model_catalog.
Queries active free models, selects a curated fallback chain,
and writes the fallback_providers section into ~/.hermes/config.yaml.

Safe: only touches the fallback_providers YAML block. Everything else stays.
Run: python3 update_hermes_fallback.py [--dry-run]

Kanban: ID 97 - Build auto-update script for Hermes fallback chain
"""

import json
import os
import re
import subprocess
import sys
import yaml

HERMES_CONFIG = os.path.expanduser("~/.hermes/config.yaml")
DB = "vibepilot"

# Provider priority order for the fallback chain
# Gemini first (fastest, most reliable free tier), then Groq, OpenRouter, NVIDIA
PROVIDER_PRIORITY = ["gemini", "groq", "openrouter", "nvidia"]

# Max models per provider in fallback chain
MAX_PER_PROVIDER = {
    "gemini": 6,
    "groq": 4,
    "openrouter": 6,
    "nvidia": 6,
}

# Models to ALWAYS exclude (known problematic or non-chat)
EXCLUDE_IDS = {
    "gemini": {
        "deep-research",       # research-only, not general chat
        "gemini-robotics",     # robotics API
        "lyria",               # music/video generation
        "gemini-flash-latest", # alias, not a real model
        "gemini-flash-lite-latest",
        "gemini-pro-latest",
        "gemini-2.0-flash-001",      # duplicate of gemini-2.0-flash
        "gemini-2.0-flash-lite-001", # duplicate of gemini-2.0-flash-lite
        "gemini-2.5-computer-use",   # computer-use beta only
        "nano-banana",               # experimental
        "gemini-3-pro",              # paid tier
        "gemini-3.1-pro",            # paid tier
        "gemini-2.0-flash",          # older gen, skip when 2.5+/3.x available
        "gemini-2.0-flash-lite",     # older gen
    },
    "groq": {
        "orpheus",  # TTS models
        "allam",    # Arabic-only
    },
    "openrouter": {
        "lyria",    # music generation, not chat
        "cohere/command-r7b",  # paid tier sneaking in as free
        "deepseek-r1",         # extremely slow, not practical fallback
    },
    "nvidia": {
        "bge",
        "baai/bge",
        "nvidia/embed",
        "intfloat",
        "snowflake",
        "cohere/embed",
        "stability",
        "sdxl",
        "playground",
        "mistralai/mixtral",
        "microsoft/phi",
    },
}


def query(sql):
    """Run SQL query against vibepilot DB, return list of dicts."""
    r = subprocess.run(
        ["psql", "-d", DB, "-t", "-A", "-F", "\t", "-c", sql],
        capture_output=True, text=True, timeout=15,
    )
    if r.returncode != 0:
        print(f"SQL error: {r.stderr}", file=sys.stderr)
        sys.exit(1)
    if not r.stdout.strip():
        return []
    result = []
    for line in r.stdout.strip().split("\n"):
        parts = line.split("\t")
        if len(parts) >= 4:
            result.append({
                "id": parts[0],
                "name": parts[1],
                "provider": parts[2],
                "capabilities": parts[3] if len(parts) > 3 else "",
            })
    return result


def should_include(model_id, provider):
    """Check if a model should be in the fallback chain."""
    model_id_lower = model_id.lower()

    # Check exclusion rules for this provider
    exclude_rules = EXCLUDE_IDS.get(provider, set())
    for rule in exclude_rules:
        if rule in model_id_lower:
            return False

    # Exclude embeddings models (not chat-capable)
    if any(x in model_id_lower for x in ["embed", "bge", "splade", "gte"]):
        return False

    # Exclude TTS/audio models
    if any(x in model_id_lower for x in ["tts", "speech", "audio", "whisper", "voice", "orpheus"]):
        return False

    # Exclude image generation models
    if any(x in model_id_lower for x in ["sdxl", "stable-diffusion", "dall-e", "flux", "playground-v2"]):
        return False

    # Exclude embedding-reranker models
    if "reranker" in model_id_lower or "re-rank" in model_id_lower:
        return False

    return True


def model_sort_key(model):
    """Sort models: reasoning|instruction > text|chat > vision > rest.
    Higher quality/capable models first."""
    caps = model["capabilities"].lower()

    # Score: higher = better for fallback
    score = 0
    if "reasoning" in caps:
        score += 100
    if "instruction" in caps:
        score += 80
    elif "text" in caps:
        score += 60
    if "code" in caps:
        score += 50
    if "vision" in caps:
        score += 10  # bonus but not primary

    # Prefer models with more capabilities listed
    cap_count = len([c for c in caps.split(",") if c.strip()])
    score += min(cap_count * 5, 30)

    return -score  # negative for descending sort


def get_hermes_model_id(db_id, provider):
    """Map DB model ID to Hermes config model string."""
    if provider == "gemini":
        # Hermes expects models/gemini-2.5-flash-lite (with models/ prefix)
        return f"models/{db_id}"
    else:
        # OpenRouter, Groq, NVIDIA use raw routing IDs
        return db_id


def fetch_active_models():
    """Fetch active free models from model_catalog, grouped by provider."""
    sql = """
    SELECT id, name, provider,
           COALESCE(array_to_string(capabilities, ','), '') AS capabilities
    FROM model_catalog
    WHERE status = 'active'
      AND is_free = true
    ORDER BY provider, id;
    """
    rows = query(sql)
    return rows


def generate_fallback_yaml(models):
    """Generate fallback_providers YAML entries from model list."""
    entries = []
    for model in models:
        entries.append({
            "model": get_hermes_model_id(model["id"], model["provider"]),
            "provider": model["provider"],
        })
    return entries


def replace_fallback_in_config(yaml_entries, dry_run=False):
    """Replace the fallback_providers section in config.yaml."""
    with open(HERMES_CONFIG, "r") as f:
        config_text = f.read()

    # Generate new fallback YAML block
    new_fallback = yaml.dump(
        yaml_entries,
        default_flow_style=False,
        sort_keys=False,
        indent=0,
        width=120,
    ).strip()

    # Build the replacement text
    new_section = f"fallback_providers:\n{new_fallback}\n"

    # Find and replace the fallback_providers section
    # Original format: dashes at column 0, provider at column 2
    pattern = r"^fallback_providers:\n(- model: .*\n  provider: .*\n?)*"
    match = re.search(pattern, config_text, re.MULTILINE)
    if not match:
        print("ERROR: Could not find fallback_providers section in config.yaml", file=sys.stderr)
        sys.exit(1)

    full_old_block = match.group(0)
    new_config_text = config_text.replace(full_old_block, new_section, 1)

    if dry_run:
        print("=== DRY RUN - would write ===")
        print(new_section)
        print("=== END ===")
        return

    # Write backup
    backup_path = HERMES_CONFIG + ".bak"
    with open(backup_path, "w") as f:
        f.write(config_text)
    print(f"Backup saved to {backup_path}")

    # Write new config
    with open(HERMES_CONFIG, "w") as f:
        f.write(new_config_text)
    print(f"Updated {HERMES_CONFIG}")

    # Validate YAML
    try:
        with open(HERMES_CONFIG, "r") as f:
            yaml.safe_load(f)
        print("Config YAML valid")
    except yaml.YAMLError as e:
        print(f"WARNING: New config YAML invalid, restoring backup: {e}", file=sys.stderr)
        with open(backup_path, "r") as f:
            os.rename(backup_path, HERMES_CONFIG)
        sys.exit(1)


def main():
    dry_run = "--dry-run" in sys.argv

    print("Fetching active free models from model_catalog...")
    all_models = fetch_active_models()
    print(f"  Found {len(all_models)} total active free models")

    # Group by provider
    by_provider = {}
    for m in all_models:
        p = m["provider"]
        if p not in by_provider:
            by_provider[p] = []
        by_provider[p].append(m)

    # Filter, sort, and select per provider
    selected = []
    for provider in PROVIDER_PRIORITY:
        pool = by_provider.get(provider, [])

        # Filter
        filtered = [m for m in pool if should_include(m["id"], provider)]

        # Sort by quality
        filtered.sort(key=model_sort_key)

        # Select top N
        max_n = MAX_PER_PROVIDER.get(provider, 5)
        chosen = filtered[:max_n]

        print(f"  {provider}: {len(pool)} total, {len(filtered)} after filter, selecting {len(chosen)}")
        for m in chosen:
            print(f"    - {m['id']} ({m.get('capabilities', '')[:40]})")

        selected.extend(chosen)

    print(f"\nTotal fallback chain: {len(selected)} models")

    # Generate YAML entries
    yaml_entries = generate_fallback_yaml(selected)

    # Write to config
    replace_fallback_in_config(yaml_entries, dry_run)

    if dry_run:
        print("\nPass --dry-run to preview. Run without to apply.")
    else:
        print("\nDone! Restart Hermes for changes to take effect.")
        print("  systemctl --user restart hermes-gateway")


if __name__ == "__main__":
    main()
