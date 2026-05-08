#!/usr/bin/env python3
"""
TokenFinder v2 — Live model discovery, capability tagging, DB persistence, and governor integration.

Scans OpenRouter, Groq, NVIDIA, and Gemini for truly free text models.
Writes to model_catalog table with capability tags, pricing, and rate limits.
Updates governor routing via DB (no rebuild needed).
Designed to run on cron: every 5 min for OpenRouter, 30 min for others.

Usage:
  python3 tokenfinder_v2.py          # scan + DB upsert
  SCAN_ONLY=openrouter python3 ...   # scan one provider only
"""

import json, os, sys, time, subprocess, urllib.request, urllib.error
from datetime import datetime, timezone

VAULT_KEY = os.environ.get("VAULT_KEY", "P9jFR25vbjcNxG2S3lx4ZCyspfGLd7wZYliZWLjqKLc=")
DB_URL = os.environ.get("DATABASE_URL", "postgres://vibes@/vibepilot?host=/var/run/postgresql")
GOVERNOR = "/home/vibes/vibepilot/governor/governor"

# Non-text models to skip
SKIP_PATTERNS = ["embedding", "image", "vision", "tts", "moderation", "rerank",
                 "whisper", "stt", "music", "video", "dalle", "stable-diffusion",
                 "sdxl", "suno", "audiocraft"]


def vault_get(key_name):
    """Get secret from PostgreSQL vault."""
    env = os.environ.copy()
    env["VAULT_KEY"] = VAULT_KEY
    env["DATABASE_URL"] = DB_URL
    r = subprocess.run([GOVERNOR, "vault", "get", key_name],
                       capture_output=True, text=True, timeout=10, env=env)
    return r.stdout.strip().split("\n")[-1].strip()


def curl_get(url, headers=None):
    cmd = ["curl", "-s", url]
    if headers:
        for k, v in headers.items():
            cmd.extend(["-H", f"{k}: {v}"])
    try:
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
        return json.loads(r.stdout) if r.returncode == 0 else {"_error": r.stderr[:200]}
    except Exception as e:
        return {"_error": str(e)}


def infer_capabilities(model_id, name=""):
    """Infer model capabilities from model ID/name using task_routing.json detection rules."""
    mid = (model_id + " " + name).lower()

    # Load detection rules from task_routing.json
    try:
        with open("/home/vibes/vibepilot/governor/config/task_routing.json") as f:
            rules = json.load(f)
    except:
        rules = {}

    caps = set()
    detection = rules.get("capability_detection", {})
    skill_rules = detection.get("rules", [])

    for rule in skill_rules:
        cap = rule["capability"]
        patterns = rule.get("detect_patterns", [])
        families = rule.get("assume_for_families", [])

        # Check name patterns
        import re
        matched = any(re.search(p, mid) for p in patterns if p)

        # Check family assumptions
        family_match = any(f.lower() in mid for f in families)

        if matched or family_match:
            caps.add(cap)

    # Always add 'text' for anything that's not excluded
    exclude_patterns = detection.get("always_exclude", {}).get("non_text_patterns", [])
    if not any(re.search(p, mid) for p in exclude_patterns):
        caps.add("text")

    return sorted(caps)


def db_upsert(models, provider):
    """Upsert models from a provider into model_catalog."""
    if not models:
        return 0

    # Check which models should stay in their current state
    # (e.g., GLM 5.1 is benched for cooldown, should not be re-activated)
    preserved = set()
    try:
        r = subprocess.run(["psql", "-d", "vibepilot", "-t", "-A",
            "-c", "SELECT id FROM model_catalog WHERE status = 'benched' AND status_reason IS NOT NULL AND status_reason != '';"],
            capture_output=True, text=True, timeout=5)
        for line in r.stdout.strip().split('\n'):
            if line.strip():
                preserved.add(line.strip())
    except:
        pass

    # Deduplicate by ID before building SQL
    seen_ids = set()
    unique_models = []
    for m in models:
        mid = m["id"]
        # Skip models that are manually benched (e.g., GLM 5.1 cooldown)
        if mid in preserved:
            continue
        if mid not in seen_ids:
            seen_ids.add(mid)
            unique_models.append(m)

    # Build bulk upsert SQL
    values = []
    for m in unique_models:
        mid = m["id"].replace("'", "''")
        name = m.get("name", m["id"]).replace("'", "''")
        ctx = m.get("context_length", 0) or 0
        is_free = "true" if m.get("is_free", True) else "false"
        p_input = m.get("pricing_input", 0) or 0
        p_output = m.get("pricing_output", 0) or 0
        caps = m.get("capabilities", ["text"])
        caps_str = "{" + ",".join(caps) + "}"
        vendor = m.get("vendor", provider)

        # Rate limits from provider defaults or model-specific
        rl = json.dumps(m.get("rate_limits", {}))
        rl_str = rl.replace("'", "''")

        values.append(
            f"('{mid}', '{name}', '{provider}', '{{{provider}}}', 'active', "
            f"'{caps_str}', {ctx}, {is_free}, {p_input}, {p_output}, "
            f"'{rl_str}'::jsonb, NOW(), NOW())"
        )

    if not values:
        return 0

    sql = """INSERT INTO model_catalog
(id, name, provider, connector_ids, status, capabilities, context_length,
 is_free, pricing_input, pricing_output, rate_limits, last_scan_at, updated_at)
VALUES
""" + ",\n".join(values) + """
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    provider = EXCLUDED.provider,
    connector_ids = CASE WHEN NOT model_catalog.connector_ids @> EXCLUDED.connector_ids
                         THEN model_catalog.connector_ids || EXCLUDED.connector_ids
                         ELSE model_catalog.connector_ids END,
    status = 'active',
    capabilities = EXCLUDED.capabilities,
    context_length = GREATEST(model_catalog.context_length, EXCLUDED.context_length),
    is_free = EXCLUDED.is_free,
    pricing_input = EXCLUDED.pricing_input,
    pricing_output = EXCLUDED.pricing_output,
    rate_limits = CASE WHEN EXCLUDED.rate_limits != '{}'::jsonb
                       THEN EXCLUDED.rate_limits ELSE model_catalog.rate_limits END,
    last_scan_at = EXCLUDED.last_scan_at,
    updated_at = NOW(),
    consecutive_failures = 0;
"""

    try:
        r = subprocess.run(["psql", "-d", "vibepilot", "-c", sql],
                          capture_output=True, text=True, timeout=60)
        if r.returncode != 0:
            print(f"  DB error: {r.stderr[:200]}")
            return 0
        return len(values)
    except Exception as e:
        print(f"  DB error: {e}")
        return 0


# ── Provider scanners ──────────────────────────────────────────

def scan_openrouter():
    """Scan OpenRouter for free text models only."""
    key = vault_get("OPENROUTER_API_KEY")
    if not key:
        print("  WARN: No OpenRouter key"); return []

    data = curl_get("https://openrouter.ai/api/v1/models",
                    {"Authorization": f"Bearer {key}"})
    if "_error" in data:
        print(f"  ERROR: {data['_error']}"); return []

    models = data.get("data", [])
    free = []
    for m in models:
        pricing = m.get("pricing", {})
        prompt = float(pricing.get("prompt", "1") or "1")
        completion = float(pricing.get("completion", "1") or "1")
        model_id = m.get("id", "")

        # ONLY free models
        if prompt != 0.0 or completion != 0.0:
            continue
        if any(s in model_id.lower() for s in SKIP_PATTERNS):
            continue
        ctx = m.get("context_length", 0) or 0
        if ctx < 4000:
            continue

        name = m.get("name", model_id)
        vendor = model_id.split("/")[0] if "/" in model_id else "openrouter"
        caps = infer_capabilities(model_id, name)

        # OpenRouter returns standard API pricing even for free models
        # Store it for ROI calculator (theoretical cost vs actual $0 paid)
        pricing_input = float(m.get("pricing", {}).get("prompt", "0") or "0")
        pricing_output = float(m.get("pricing", {}).get("completion", "0") or "0")

        # OpenRouter free tier rate limits
        rate_limits = {"rpd": 200, "rpm": 20}

        free.append({
            "id": model_id, "name": name, "provider": "openrouter",
            "context_length": ctx, "is_free": True,
            "pricing_input": pricing_input, "pricing_output": pricing_output,
            "vendor": vendor, "capabilities": caps,
            "rate_limits": rate_limits,
        })

    return free


def scan_groq():
    """Scan Groq — all models are free."""
    key = vault_get("GROQ_API_KEY")
    if not key:
        print("  WARN: No Groq key"); return []

    data = curl_get("https://api.groq.com/openai/v1/models",
                    {"Authorization": f"Bearer {key}"})
    if "_error" in data:
        print(f"  ERROR: {data['_error']}"); return []

    models = data.get("data", [])
    available = []
    for m in models:
        model_id = m.get("id", "")
        ctx = m.get("context_window", 0) or 0
        active = m.get("active", True)
        if not active or any(s in model_id.lower() for s in SKIP_PATTERNS) or ctx < 4000:
            continue
        full_id = f"groq/{model_id}" if "/" not in model_id else model_id
        caps = infer_capabilities(model_id)
        rate_limits = {"rpm": 30, "rpd": 14400}

        available.append({
            "id": full_id, "name": model_id, "provider": "groq",
            "context_length": ctx, "is_free": True,
            "pricing_input": 0, "pricing_output": 0,
            "vendor": "groq", "capabilities": caps,
            "rate_limits": rate_limits,
        })

    return available


def scan_nvidia():
    """Scan NVIDIA NIM — all free tier models are at no cost."""
    key = vault_get("NVIDIA_API_KEY")
    if not key:
        print("  WARN: No NVIDIA key"); return []

    data = curl_get("https://integrate.api.nvidia.com/v1/models",
                    {"Authorization": f"Bearer {key}"})
    if "_error" in data:
        print(f"  ERROR: {data['_error']}"); return []

    models = data.get("data", [])
    available = []
    for m in models:
        model_id = m.get("id", "")
        if any(s in model_id.lower() for s in SKIP_PATTERNS):
            continue

        # NVIDIA doesn't return prices or context in list endpoint
        # All models on NVIDIA's free NIM tier are free to use
        ctx = 128000  # Default — all NIM models support >= 128K
        full_id = f"nvidia/{model_id}" if "/" not in model_id else model_id
        caps = infer_capabilities(model_id)
        
        # Skip text-only models with small context (likely embeddings/specialized)
        if caps == ["text"] and ctx < 8000:
            continue
            
        rate_limits = {"rpm": 10, "rpd": 1000}

        available.append({
            "id": full_id, "name": model_id, "provider": "nvidia",
            "context_length": ctx, "is_free": True,
            "pricing_input": 0, "pricing_output": 0,
            "vendor": model_id.split("/")[0] if "/" in model_id else "nvidia",
            "capabilities": caps,
            "rate_limits": rate_limits,
        })

    return available


def scan_gemini():
    """Scan Gemini using all 4 API keys — all models are free tier."""
    all_models = {}
    for key_env in ["GEMINI_KEY_1", "GEMINI_KEY_2", "GEMINI_KEY_3", "GEMINI_KEY_4"]:
        key = vault_get(key_env)
        if not key:
            try:
                with open("/home/vibes/.hermes/.env") as f:
                    for line in f:
                        if line.startswith(f"{key_env}="):
                            key = line.split("=", 1)[1].strip().strip('"\'')
                            break
            except:
                pass
        if not key:
            print(f"  WARN: {key_env} not found"); continue

        data = curl_get(f"https://generativelanguage.googleapis.com/v1beta/models?key={key}")
        if "_error" in data:
            continue

        for m in data.get("models", []):
            model_id = m.get("name", "")
            methods = m.get("supportedGenerationMethods", [])
            if "generateContent" not in methods:
                continue
            if any(s in model_id.lower() for s in SKIP_PATTERNS):
                continue
            short_id = model_id.replace("models/", "", 1)
            ctx = m.get("inputTokenLimit", 0) or 0

            caps = infer_capabilities(short_id, m.get("displayName", ""))
            caps.append("vision")  # All Gemini models support vision
            caps = list(set(caps))

            # Gemini free tier: 15 RPM per key (60 RPM across 4 keys)
            rate_limits = {"rpm_per_key": 15, "total_rpm": 60, "rpd_per_key": 1500}

            if short_id not in all_models:
                all_models[short_id] = {
                    "id": short_id, "name": m.get("displayName", short_id),
                    "provider": "google", "context_length": ctx,
                    "is_free": True, "pricing_input": 0, "pricing_output": 0,
                    "vendor": "google", "capabilities": caps,
                    "rate_limits": rate_limits,
                }

    return list(all_models.values())


def update_scan_state(provider, status, count, ms):
    """Record scan result in provider_scan_state."""
    sql = f"""INSERT INTO provider_scan_state
VALUES ('{provider}', NOW(), '{status}', {count}, 0, {ms}, NULL, NOW())
ON CONFLICT (provider) DO UPDATE SET
    last_scan_at = NOW(), last_scan_status = '{status}',
    models_found = {count}, scan_duration_ms = {ms}, updated_at = NOW();"""
    subprocess.run(["psql", "-d", "vibepilot", "-c", sql],
                   capture_output=True, timeout=10)


def main():
    scan_filter = os.environ.get("SCAN_ONLY", "")

    print("=" * 60)
    print(f"TokenFinder v2 — {datetime.now(timezone.utc).isoformat()}")
    print("=" * 60)

    # Record scan start time for cleanup
    scan_start = datetime.now(timezone.utc)

    total_upserted = 0

    scans = []
    if not scan_filter or scan_filter == "openrouter":
        scans.append(("OpenRouter", scan_openrouter, "openrouter"))
    if not scan_filter or scan_filter == "groq":
        scans.append(("Groq", scan_groq, "groq"))
    if not scan_filter or scan_filter == "nvidia":
        scans.append(("NVIDIA", scan_nvidia, "nvidia"))
    if not scan_filter or scan_filter == "gemini":
        scans.append(("Gemini", scan_gemini, "gemini"))

    for name, scan_fn, provider in scans:
        print(f"\n--- {name} ---")
        t1 = time.time()
        models = scan_fn()
        dt = int((time.time() - t1) * 1000)
        print(f"  Found: {len(models)} free models ({dt}ms)")

        if models:
            upserted = db_upsert(models, provider)
            total_upserted += upserted
            print(f"  Upserted: {upserted} to model_catalog")

            # Show capabilities breakdown
            cap_counts = {}
            for m in models:
                for c in m.get("capabilities", []):
                    cap_counts[c] = cap_counts.get(c, 0) + 1
            print(f"  Capabilities: {cap_counts}")

        update_scan_state(provider, "ok" if models else "empty", len(models), dt)

    print(f"\n{'=' * 60}")
    print(f"Total: {total_upserted} models upserted to model_catalog")

    # ── Credit Balance Check (paid APIs, runs every 30 min) ──
    print(f"\n--- Credit Balance Check ---")
    for provider_name, key_env in [("DeepSeek", "DEEPSEEK_V4_FLASH_KEY")]:
        api_key = os.environ.get(key_env, "")
        if not api_key:
            api_key = vault_get(key_env) or ""
        if api_key:
            try:
                req = urllib.request.Request(
                    f"https://api.deepseek.com/user/balance",
                    headers={"Authorization": f"Bearer {api_key}", "User-Agent": "VibePilot-TokenFinder/2.0"}
                )
                with urllib.request.urlopen(req, timeout=10) as resp:
                    data = json.loads(resp.read())
                if data.get("is_available"):
                    total_bal = sum(float(i.get("total_balance", 0)) for i in data.get("balance_infos", []))
                    print(f"  {provider_name}: ${total_bal:.2f} remaining")

                    import psycopg
                    conn = psycopg.connect("dbname=vibepilot host=/var/run/postgresql")
                    cur = conn.execute(
                        "SELECT SUM(credit_total_usd) FROM models WHERE id LIKE %s AND credit_total_usd > 0",
                        ("%deepseek%",)
                    )
                    row = cur.fetchone()
                    total_cred = float(row[0]) if row and row[0] else 10.0
                    if total_cred > 0:
                        ratio = total_bal / total_cred
                        conn.execute(
                            "UPDATE models SET credit_remaining_usd = %s * (credit_total_usd / %s) "
                            "WHERE id LIKE %s AND credit_total_usd > 0",
                            (total_bal, total_cred, "%deepseek%")
                        )
                        if ratio < 0.2:
                            print(f"  ⚠️  {provider_name} at {ratio*100:.0f}% remaining (${total_bal:.2f} of ${total_cred:.2f})")
                        elif ratio < 0.5:
                            print(f"  {provider_name} at {ratio*100:.0f}% remaining")
                    conn.commit()
                    conn.close()
            except Exception as e:
                print(f"  {provider_name} credit check error: {e}")
        else:
            print(f"  No API key for {provider_name}")

    # Clean up: bench old entries not updated by this scan
    print(f"\n--- Cleanup: benching old entries ---")
    cutoff = scan_start.isoformat()
    cleanup_sql = f"UPDATE model_catalog SET status = 'benched', updated_at = NOW() WHERE status = 'active' AND (last_scan_at IS DISTINCT FROM NULL AND last_scan_at < '{cutoff}');"
    cr = subprocess.run(["psql", "-d", "vibepilot", "-c", cleanup_sql], capture_output=True, text=True, timeout=30)
    if cr.returncode == 0:
        for line in cr.stdout.split('\n'):
            if 'UPDATE' in line:
                print(f"  {line.strip()} old entries benched")
    else:
        print(f"  Cleanup error: {cr.stderr[:100]}")

    # Write live snapshot to GitHub repo
    print("\n--- Writing live model configs to repo ---")
    snapshot = []
    for provider in ["openrouter", "groq", "nvidia", "gemini"]:
        r = subprocess.run(["psql", "-d", "vibepilot", "-t", "-A", "-F", "|",
            "-c", f"SELECT id, name, context_length, capabilities::text, rate_limits::text, connector_ids::text FROM model_catalog WHERE provider = '{provider}' AND status = 'active' ORDER BY context_length DESC;"],
            capture_output=True, text=True, timeout=10)
        for line in r.stdout.strip().split('\n'):
            if not line.strip():
                continue
            parts = line.split('|')
            if len(parts) >= 6:
                snapshot.append({
                    "id": parts[0], "name": parts[1],
                    "context_length": int(parts[2]) if parts[2].isdigit() else 0,
                    "capabilities": parts[3].strip('{}').split(',') if parts[3] != '{}' else [],
                    "rate_limits": json.loads(parts[4]) if parts[4] else {},
                    "provider": provider,
                    "connector_ids": parts[5].strip('{}').split(',') if parts[5] != '{}' else []
                })

    # Write models_live.json (human-readable snapshot)
    with open("/home/vibes/vibepilot/governor/config/models_live.json", "w") as f:
        json.dump({"version": 2, "updated": datetime.now(timezone.utc).isoformat(),
                   "total": len(snapshot), "models": snapshot}, f, indent=2)
    print(f"  Wrote {len(snapshot)} models to governor/config/models_live.json")

    go_models = []
    
    # Query models table for existing status overrides
    existing_status = {}
    try:
        r = subprocess.run(["psql", "-d", "vibepilot", "-t", "-A", "-F", "|",
            "-c", "SELECT id, status, status_reason FROM models WHERE status != 'active';"],
            capture_output=True, text=True, timeout=5)
        for line in r.stdout.strip().split('\n'):
            if not line.strip():
                continue
            parts = line.split('|')
            if len(parts) >= 2:
                existing_status[parts[0]] = {"status": parts[1], "reason": parts[2] if len(parts) >= 3 else ""}
    except:
        pass
    
    # Map short provider names to actual connector IDs
    provider_to_connector = {
        "openrouter": ["openrouter-api"],
        "groq": ["groq-api"],
        "nvidia": ["nvidia-api"],
        "google": ["gemini-api-general"],
        "gemini": ["gemini-api-general"],
    }
    
    for m in snapshot:
        # Map connector_ids to actual connector IDs
        raw_ids = m.get("connector_ids", [])
        access_via = []
        for cid in raw_ids:
            mapped = provider_to_connector.get(cid, [cid + "-api" if not cid.endswith("-api") else cid])
            access_via.extend(mapped if isinstance(mapped, list) else [mapped])
        if not access_via:
            access_via = provider_to_connector.get(m["provider"], [m["provider"] + "-api"])
        
        # Preserve status from models table if available (e.g., bench/cooldown)
        model_status = "active"
        if m["id"] in existing_status:
            model_status = existing_status[m["id"]]["status"]
        
        go_models.append({
            "id": m["id"],
            "name": m["name"],
            "provider": m["provider"],
            "context_limit": m["context_length"],
            "capabilities": m["capabilities"],
            "access_via": access_via,
            "status": model_status,
        })

    # Rename old models.json to .legacy, write new one
    legacy = "/home/vibes/vibepilot/governor/config/models.json.legacy"
    current = "/home/vibes/vibepilot/governor/config/models.json"
    if os.path.exists(current) and not os.path.exists(legacy):
        os.rename(current, legacy)
        print(f"  Renamed old models.json -> models.json.legacy")

    with open(current, "w") as f:
        json.dump({"models": go_models}, f, indent=2)
    print(f"  Wrote {len(go_models)} models to governor/config/models.json")

    # Commit and push both files to GitHub
    git_files = ["governor/config/models_live.json", "governor/config/models.json"]
    if os.path.exists(legacy):
        git_files.append("governor/config/models.json.legacy")
    
    for gf in git_files:
        subprocess.run(["git", "-C", "/home/vibes/vibepilot", "add", gf],
                      capture_output=True, timeout=10)
    r2 = subprocess.run(["git", "-C", "/home/vibes/vibepilot", "diff", "--cached", "--quiet"],
                       capture_output=True, timeout=10)
    if r2.returncode != 0:
        try:
            subprocess.run(["git", "-C", "/home/vibes/vibepilot", "commit", "-m",
                           f"live model config: {len(go_models)} models from model_catalog"],
                           capture_output=True, timeout=60)
            subprocess.run(["git", "-C", "/home/vibes/vibepilot", "push", "origin", "main"],
                          capture_output=True, timeout=60)
            print(f"  Pushed to GitHub")
        except subprocess.TimeoutExpired:
            print("  Git commit/push timed out — will retry next scan")
    else:
        print("  No changes since last snapshot")

    print("Done")


if __name__ == "__main__":
    main()
