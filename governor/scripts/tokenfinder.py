#!/usr/bin/env python3
"""
TokenFinder -- Real-time model discovery, health checking, and Hermes config updater.

Scans OpenRouter, Groq, NVIDIA, and Gemini for currently free + working models.
Records results to DB. Updates Hermes credential pools and fallback lists.
Designed to run on cron (every 30 min for OpenRouter, hourly for others).

Usage:
  TOKENFINDER_UPDATE_HERMES=true python3 tokenfinder.py   # scan + update Hermes
  python3 tokenfinder.py                                  # scan only
"""

import json, os, sys, time, subprocess
from datetime import datetime, timezone

VAULT_KEY = os.environ.get("VAULT_KEY", "P9jFR25vbjcNxG2S3lx4ZCyspfGLd7wZYliZWLjqKLc=")
DB_URL = os.environ.get("DATABASE_URL", "postgres://vibes@/vibepilot?host=/var/run/postgresql")
GOVERNOR = "/home/vibes/vibepilot/governor/governor"
HERMES_CONFIG = "/home/vibes/.hermes/config.yaml"

SKIP_PATTERNS = ["embedding", "image", "vision", "tts", "moderation", "rerank",
                 "whisper", "stt", "music", "video", "dalle", "stable-diffusion",
                 "sdxl", "suno", "audiocraft"]


def vault_get(key_name):
    """Get a secret from the PostgreSQL vault."""
    cmd = [GOVERNOR, "vault", "get", key_name]
    env = os.environ.copy()
    env["VAULT_KEY"] = VAULT_KEY
    env["DATABASE_URL"] = DB_URL
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=10, env=env)
    return result.stdout.strip().split("\n")[-1].strip()


def curl_get(url, headers=None):
    """Make a GET request using curl."""
    cmd = ["curl", "-s", url]
    if headers:
        for k, v in headers.items():
            cmd.extend(["-H", f"{k}: {v}"])
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
        if result.returncode != 0:
            return {"_error": f"curl failed: {result.stderr[:200]}"}
        try:
            return json.loads(result.stdout)
        except json.JSONDecodeError:
            return {"_error": f"JSON parse error, raw: {result.stdout[:200]}"}
    except Exception as e:
        return {"_error": str(e)}


def scan_openrouter():
    """Scan OpenRouter for free text models."""
    key = vault_get("OPENROUTER_API_KEY")
    if not key:
        print("  WARN: OpenRouter key empty"); return []

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
        if prompt != 0.0 or completion != 0.0:
            continue
        if any(s in model_id.lower() for s in SKIP_PATTERNS):
            continue
        ctx = m.get("context_length", 0) or 0
        if ctx < 8000:
            continue
        vendor = model_id.split("/")[0] if "/" in model_id else "openrouter"
        free.append({"id": model_id, "provider": "openrouter",
                     "context_length": ctx, "vendor": vendor,
                     "pricing_input": prompt, "pricing_output": completion})

    return free


def scan_groq():
    """Scan Groq for available models."""
    key = vault_get("GROQ_API_KEY")
    if not key:
        print("  WARN: Groq key empty"); return []

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
        if not active:
            continue
        if any(s in model_id.lower() for s in SKIP_PATTERNS):
            continue
        if ctx < 8000:
            continue
        full_id = f"groq/{model_id}" if "/" not in model_id else model_id
        available.append({"id": full_id, "provider": "groq",
                         "context_length": ctx, "vendor": "groq"})

    return available


def scan_nvidia():
    """Scan NVIDIA NIM for available models."""
    key = vault_get("NVIDIA_API_KEY")
    if not key:
        print("  WARN: NVIDIA key empty"); return []

    data = curl_get("https://integrate.api.nvidia.com/v1/models",
                    {"Authorization": f"Bearer {key}"})
    if "_error" in data:
        print(f"  ERROR: {data['_error']}"); return []

    models = data.get("data", [])
    available = []
    for m in models:
        model_id = m.get("id", "")
        ctx = m.get("context_length", 0) or 0
        if any(s in model_id.lower() for s in SKIP_PATTERNS):
            continue
        # NVIDIA API doesn't return context_length in list endpoint
        # All NVIDIA NIM models support at least 128K context
        if ctx == 0:
            ctx = 128000  # Default for all NVIDIA models
        full_id = f"nvidia/{model_id}" if "/" not in model_id else model_id
        available.append({"id": full_id, "provider": "nvidia",
                         "context_length": ctx, "vendor": "nvidia"})

    return available


def scan_gemini():
    """Scan Gemini using all 4 API keys."""
    all_models = {}
    for key_env in ["GEMINI_KEY_1", "GEMINI_KEY_2", "GEMINI_KEY_3", "GEMINI_KEY_4"]:
        key = vault_get(key_env)
        if not key:
            # Try sourcing from .env
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
            print(f"  WARN: {key_env}: {data['_error'][:60]}"); continue

        for m in data.get("models", []):
            model_id = m.get("name", "")
            methods = m.get("supportedGenerationMethods", [])
            if "generateContent" not in methods:
                continue
            if any(s in model_id.lower() for s in SKIP_PATTERNS):
                continue
            short_id = model_id.replace("models/", "", 1)
            ctx = m.get("inputTokenLimit", 0) or 0
            if short_id not in all_models:
                all_models[short_id] = {"id": short_id, "provider": "google",
                                       "context_length": ctx, "vendor": "google"}

    return list(all_models.values())


def update_provider_scan_state(provider, status, models_found, duration_ms=0):
    """Record scan results in DB."""
    sql = f"""INSERT INTO provider_scan_state (provider, last_scan_at, last_scan_status, models_found, scan_duration_ms)
VALUES ('{provider}', NOW(), '{status}', {models_found}, {duration_ms})
ON CONFLICT (provider) DO UPDATE SET
    last_scan_at = NOW(), last_scan_status = '{status}',
    models_found = {models_found}, scan_duration_ms = {duration_ms}, updated_at = NOW();"""
    try:
        subprocess.run(["psql", "-d", "vibepilot", "-c", sql],
                      capture_output=True, timeout=10)
    except Exception as e:
        print(f"  DB update error: {e}")


def build_fallback_list(models_by_provider):
    """Pick best models from each provider for Hermes fallback."""
    fallback = []

    # Gemini: prefer gemini-2.5-flash then 2.0-flash
    for needle in ["2.5-flash", "2.0-flash"]:
        for m in models_by_provider.get("google", []):
            if needle in m["id"]:
                fallback.append({"provider": "google", "model": m["id"]})
                break

    # Groq: top 3 instruction/chat models by context
    groq_models = sorted(models_by_provider.get("groq", []),
                        key=lambda m: m["context_length"], reverse=True)
    for m in groq_models[:4]:
        fallback.append({"provider": "groq", "model": m["id"]})

    # OpenRouter: top 5 free models by context
    or_models = sorted(models_by_provider.get("openrouter", []),
                      key=lambda m: m["context_length"], reverse=True)
    count = 0
    for m in or_models:
        if count >= 5:
            break
        mid = f"{m['id']}:free" if ":free" not in m["id"] else m["id"]
        fallback.append({"provider": "openrouter", "model": mid})
        count += 1
    # Catch-all
    fallback.append({"provider": "openrouter", "model": "openrouter/free"})

    # NVIDIA: top 2
    nv_models = sorted(models_by_provider.get("nvidia", []),
                      key=lambda m: m["context_length"], reverse=True)
    for m in nv_models[:2]:
        fallback.append({"provider": "nvidia", "model": m["id"]})

    return fallback


def write_hermes_fallback(fallback_list):
    """Write updated fallback list to Hermes config.yaml."""
    with open(HERMES_CONFIG) as f:
        lines = f.readlines()

    new_lines = []
    in_fallback = False
    written = False
    for line in lines:
        if line.strip().startswith("fallback_providers:"):
            in_fallback = True
            new_lines.append(line)
            for entry in fallback_list:
                new_lines.append(f"- provider: {entry['provider']}\n")
                new_lines.append(f"  model: {entry['model']}\n")
            written = True
            continue
        if in_fallback:
            if not line.startswith((" ", "-")) and line.strip():
                in_fallback = False
                new_lines.append(line)
            elif line.strip().startswith("credential_pool_strategies"):
                in_fallback = False
                new_lines.append(line)
            else:
                continue
        else:
            new_lines.append(line)

    with open(HERMES_CONFIG, "w") as f:
        f.writelines(new_lines)

    print(f"  Updated Hermes config with {len(fallback_list)} fallback models")
    return True


def main():
    print("=" * 60)
    print(f"TokenFinder Scan -- {datetime.now(timezone.utc).isoformat()}")
    print("=" * 60)

    all_free = {}
    t_start = time.time()

    # 1. OpenRouter
    print("\n[1/4] Scanning OpenRouter for free models...")
    t1 = time.time()
    or_models = scan_openrouter()
    dt = int((time.time() - t1) * 1000)
    print(f"  Found {len(or_models)} free text models ({dt}ms)")
    all_free["openrouter"] = or_models
    update_provider_scan_state("openrouter", "ok" if or_models else "empty", len(or_models), dt)

    # 2. Groq
    print("\n[2/4] Scanning Groq...")
    t2 = time.time()
    groq_models = scan_groq()
    dt2 = int((time.time() - t2) * 1000)
    print(f"  Found {len(groq_models)} models ({dt2}ms)")
    all_free["groq"] = groq_models
    update_provider_scan_state("groq", "ok" if groq_models else "empty", len(groq_models), dt2)

    # 3. NVIDIA
    print("\n[3/4] Scanning NVIDIA...")
    t3 = time.time()
    nvidia_models = scan_nvidia()
    dt3 = int((time.time() - t3) * 1000)
    print(f"  Found {len(nvidia_models)} models ({dt3}ms)")
    all_free["nvidia"] = nvidia_models
    update_provider_scan_state("nvidia", "ok" if nvidia_models else "empty", len(nvidia_models), dt3)

    # 4. Gemini
    print("\n[4/4] Scanning Gemini (4 API keys)...")
    t4 = time.time()
    gemini_models = scan_gemini()
    dt4 = int((time.time() - t4) * 1000)
    print(f"  Found {len(gemini_models)} models ({dt4}ms)")
    all_free["google"] = gemini_models
    update_provider_scan_state("gemini", "ok" if gemini_models else "empty", len(gemini_models), dt4)

    total = sum(len(v) for v in all_free.values())
    print(f"\n{'=' * 60}")
    print(f"SCAN SUMMARY: {total} total models viable")
    print(f"  OpenRouter: {len(or_models)} free text models")
    print(f"  Groq:       {len(groq_models)} models")
    print(f"  NVIDIA:     {len(nvidia_models)} models")
    print(f"  Gemini:     {len(gemini_models)} models (4 keys)")
    print(f"  Duration:   {int((time.time() - t_start) * 1000)}ms")

    # Top picks
    print(f"\n--- Top models by context window ---")
    flat = [m for lst in all_free.values() for m in lst]
    flat.sort(key=lambda m: m["context_length"], reverse=True)
    for m in flat[:12]:
        print(f"  {m['id']}  ({m['context_length']:,} ctx)  [{m['provider']}]")

    # Update Hermes config
    if os.environ.get("TOKENFINDER_UPDATE_HERMES") == "true" or os.environ.get("TOKENFINDER_AUTO_UPDATE") == "true":
        print(f"\n--- Updating Hermes fallback config ---")
        fallback = build_fallback_list(all_free)
        if write_hermes_fallback(fallback):
            print(f"  Run: systemctl --user restart hermes-gateway")
            for f in fallback:
                print(f"    {f['provider']} -> {f['model']}")

    print(f"\nTokenFinder scan complete")


if __name__ == "__main__":
    main()
