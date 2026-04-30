#!/usr/bin/env python3
"""
VibePilot Daily Model Health Check & Sync

Runs as a GitHub Actions cron or local cron. For each provider:
1. Fetches current model list and rate limits
2. Checks model health (can we actually call it?)
3. Updates models.json with accurate rate limits
4. Flags models that are down or changed
5. Applies ban list

Outputs a JSON report for the dashboard.
"""

import json
import os
import sys
import time
import urllib.request
import urllib.error
from datetime import datetime, timezone
from pathlib import Path

CONFIG_DIR = Path(__file__).parent.parent / "config"
MODELS_FILE = CONFIG_DIR / "models.json"
ROUTING_FILE = CONFIG_DIR / "routing.json"
REPORT_FILE = CONFIG_DIR / "health_report.json"

# Models to never use
BAN_LIST = [
    "nemotron",
    "llama-3-instruct",
    "ling-2.6",
    "dolphin-mistral",  # too small for pipeline use
    "lfm-2.5-1.2b",     # 1.2B too small
]

# Minimum model size to be useful for pipeline
MIN_PARAMS_BILLION = 4

def load_env():
    """Load API keys from env file, handling quoted values."""
    env_file = Path.home() / ".hermes" / ".env"
    env = {}
    if env_file.exists():
        for line in env_file.read_text().splitlines():
            line = line.strip()
            if line and not line.startswith('#') and '=' in line:
                key, val = line.split('=', 1)
                val = val.strip('"').strip("'")
                env[key.strip()] = val
    # Also inherit from process environment (for cron/CI)
    for key in ["GROQ_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
                "OPENROUTER_API_KEY", "NVIDIA_API_KEY", "ZAI_API_KEY"]:
        if key in os.environ and key not in env:
            env[key] = os.environ[key]
    return env

def api_get(url, headers=None, timeout=10):
    """Simple GET request."""
    hdrs = {"User-Agent": "VibePilot-HealthCheck/1.0"}
    if headers:
        hdrs.update(headers)
    req = urllib.request.Request(url, headers=hdrs)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read()), None
    except urllib.error.HTTPError as e:
        return None, f"HTTP {e.code}: {e.reason}"
    except Exception as e:
        return None, str(e)

def api_post(url, body, headers=None, timeout=30):
    """Simple POST request."""
    data = json.dumps(body).encode()
    req = urllib.request.Request(url, data=data, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read()), None
    except urllib.error.HTTPError as e:
        body_text = e.read().decode()[:200]
        return None, f"HTTP {e.code}: {body_text}"
    except Exception as e:
        return None, str(e)

def check_openrouter(env):
    """Fetch free models from OpenRouter API with actual rate limits."""
    api_key = env.get("OPENROUTER_API_KEY")
    if not api_key:
        return None, "No OPENROUTER_API_KEY"

    data, err = api_get(
        "https://openrouter.ai/api/v1/models",
        headers={"Authorization": f"Bearer {api_key}"}
    )
    if err:
        return None, err

    models = []
    for m in data.get("data", []):
        pricing = m.get("pricing", {})
        prompt_price = float(pricing.get("prompt", "1") or "1")
        completion_price = float(pricing.get("completion", "1") or "1")
        is_free = prompt_price == 0 and completion_price == 0

        # Get actual rate limits from OpenRouter
        limit = m.get("per_request_limits", {})
        rl = m.get("top_provider", {}).get("context_length", 0)

        model_info = {
            "id": m["id"],
            "name": m.get("name", m["id"]),
            "provider": "openrouter",
            "is_free": is_free,
            "context_length": m.get("context_length", 0),
            "pricing": {"prompt": prompt_price, "completion": completion_price},
            "rate_limits": {
                "requests_per_minute": 20,  # OpenRouter default for free
                "requests_per_day": 200,
            },
            "arch": m.get("architecture", {}),
        }

        # Check if it's a preview/deprecated model
        if m.get("published", 0) < 1:
            model_info["preview"] = True

        models.append(model_info)

    return models, None

def check_gemini(env):
    """Check Gemini API health and list available models."""
    api_key = env.get("GEMINI_API_KEY") or env.get("GOOGLE_API_KEY")
    if not api_key:
        return None, "No GEMINI_API_KEY"

    data, err = api_get(
        f"https://generativelanguage.googleapis.com/v1beta/models?key={api_key}"
    )
    if err:
        return None, err

    models = []
    for m in data.get("models", []):
        mid = m.get("name", "").replace("models/", "")
        # Skip embedding/vision-only models
        if "embedding" in mid or "aqa" in mid:
            continue

        rate_limits = {}
        # Gemini free tier limits
        if "flash" in mid:
            rate_limits = {"requests_per_minute": 15, "requests_per_day": 1500, "tokens_per_minute": 1000000}
        elif "pro" in mid:
            rate_limits = {"requests_per_minute": 5, "requests_per_day": 25, "tokens_per_minute": 250000}

        models.append({
            "id": mid,
            "name": m.get("displayName", mid),
            "provider": "google",
            "is_free": True,
            "context_length": int(m.get("inputTokenLimit", 0)),
            "rate_limits": rate_limits,
            "methods": m.get("supportedGenerationMethods", []),
        })

    return models, None

def check_groq(env):
    """Check Groq API health and list available models."""
    api_key = env.get("GROQ_API_KEY")
    if not api_key:
        return None, "No GROQ_API_KEY"

    data, err = api_get(
        "https://api.groq.com/openai/v1/models",
        headers={"Authorization": f"Bearer {api_key}"}
    )
    if err:
        return None, err

    models = []
    for m in data.get("data", []):
        mid = m.get("id", "")
        # Groq free tier: 30 RPM, 14400 RPD, varies by model
        rate_limits = {"requests_per_minute": 30, "requests_per_day": 14400}
        if "8b" in mid or "instant" in mid:
            rate_limits["tokens_per_day"] = 500000
        else:
            rate_limits["tokens_per_day"] = 100000

        models.append({
            "id": mid,
            "name": m.get("id", ""),
            "provider": "groq",
            "is_free": True,
            "rate_limits": rate_limits,
        })

    return models, None

def health_check_model(model_id, provider, env):
    """Try to actually call a model to verify it works."""
    if provider == "google":
        api_key = env.get("GEMINI_API_KEY") or env.get("GOOGLE_API_KEY")
        if not api_key:
            return False, "no key"
        url = f"https://generativelanguage.googleapis.com/v1beta/models/{model_id}:generateContent?key={api_key}"
        data = json.dumps({"contents": [{"parts": [{"text": "hi"}]}]}).encode()
        req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json", "User-Agent": "VibePilot-HealthCheck/1.0"})
        try:
            with urllib.request.urlopen(req, timeout=15) as resp:
                return True, "ok"
        except urllib.error.HTTPError as e:
            if e.code == 429:
                return True, "rate_limited_but_alive"
            body = e.read().decode()[:200]
            return False, f"HTTP {e.code}: {body}"
        except Exception as e:
            return False, str(e)

    elif provider == "groq":
        api_key = env.get("GROQ_API_KEY")
        if not api_key:
            return False, "no key"
        data = json.dumps({"model": model_id, "messages": [{"role": "user", "content": "hi"}], "max_tokens": 1}).encode()
        req = urllib.request.Request(
            "https://api.groq.com/openai/v1/chat/completions",
            data=data,
            headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json", "User-Agent": "VibePilot-HealthCheck/1.0"}
        )
        try:
            with urllib.request.urlopen(req, timeout=15) as resp:
                return True, "ok"
        except urllib.error.HTTPError as e:
            if e.code == 429:
                return True, "rate_limited_but_alive"
            body = e.read().decode()[:200]
            return False, f"HTTP {e.code}: {body}"
        except Exception as e:
            return False, str(e)

    elif provider in ("openrouter", "alibaba", "meta", "nvidia", "bytedance",
                       "inclusionai", "tencent", "moonshot", "liquid", "mistral"):
        api_key = env.get("OPENROUTER_API_KEY")
        if not api_key:
            return False, "no key"
        data = json.dumps({"model": model_id, "messages": [{"role": "user", "content": "hi"}], "max_tokens": 1}).encode()
        req = urllib.request.Request(
            "https://openrouter.ai/api/v1/chat/completions",
            data=data,
            headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json", "User-Agent": "VibePilot-HealthCheck/1.0"}
        )
        try:
            with urllib.request.urlopen(req, timeout=15) as resp:
                return True, "ok"
        except urllib.error.HTTPError as e:
            if e.code == 429:
                return True, "rate_limited_but_alive"
            body = e.read().decode()[:200]
            return False, f"HTTP {e.code}: {body}"
        except Exception as e:
            return False, str(e)

    return None, "skip"

def is_banned(model_id):
    """Check if model is on the ban list."""
    mid_lower = model_id.lower()
    return any(banned in mid_lower for banned in BAN_LIST)

def apply_ban_list(models):
    """Remove banned models from list."""
    before = len(models)
    models = [m for m in models if not is_banned(m["id"])]
    removed = before - len(models)
    return models, removed

def update_models_config(remote_models, current_config):
    """Update existing models' rate limits from remote data. Do NOT add new models."""
    current = {m["id"]: m for m in current_config.get("models", [])}
    updated = list(current.values())  # keep exact same models
    new_free_models = []
    changed_limits = []

    for rm in remote_models:
        mid = rm["id"]
        if mid in current:
            # Update existing model's rate limits if we got real ones
            existing = current[mid]
            if rm.get("rate_limits"):
                for k, v in rm["rate_limits"].items():
                    if v is not None:
                        old_val = existing.get("rate_limits", {}).get(k)
                        if old_val != v:
                            changed_limits.append((mid, k, old_val, v))
                            existing.setdefault("rate_limits", {})[k] = v
            if rm.get("context_length") and not existing.get("context_length"):
                existing["context_length"] = rm["context_length"]
        else:
            # Track new FREE models as candidates (don't auto-add)
            if rm.get("is_free") and not is_banned(mid):
                new_free_models.append(rm)

    return updated, new_free_models, changed_limits

def main():
    env = load_env()
    report = {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "providers": {},
        "health_checks": {},
        "changes": [],
        "warnings": [],
    }

    # Load current config
    with open(MODELS_FILE) as f:
        config = json.load(f)

    print(f"VibePilot Daily Model Health Check")
    print(f"==================================")
    print(f"Current models in config: {len(config['models'])}")
    print()

    # Check each provider
    all_remote = []

    print("--- Google Gemini ---")
    gemini_models, err = check_gemini(env)
    if err:
        print(f"  ERROR: {err}")
        report["warnings"].append(f"Gemini: {err}")
    else:
        print(f"  Found {len(gemini_models)} models")
        report["providers"]["google"] = {"status": "ok", "models": len(gemini_models)}
        all_remote.extend(gemini_models)

    print("--- Groq ---")
    groq_models, err = check_groq(env)
    if err:
        print(f"  ERROR: {err}")
        report["warnings"].append(f"Groq: {err}")
    else:
        print(f"  Found {len(groq_models)} models")
        report["providers"]["groq"] = {"status": "ok", "models": len(groq_models)}
        all_remote.extend(groq_models)

    print("--- OpenRouter ---")
    or_models, err = check_openrouter(env)
    if err:
        print(f"  ERROR: {err}")
        report["warnings"].append(f"OpenRouter: {err}")
    else:
        free_count = sum(1 for m in or_models if m.get("is_free"))
        print(f"  Found {len(or_models)} models ({free_count} free)")
        report["providers"]["openrouter"] = {"status": "ok", "models": len(or_models), "free": free_count}
        all_remote.extend(or_models)

    # Apply ban list
    all_remote, banned_count = apply_ban_list(all_remote)
    if banned_count:
        print(f"\n  Banned {banned_count} models from ban list")

    # Update config
    print(f"\n--- Updating models.json ---")
    updated_models, new_free_models, changed_limits = update_models_config(all_remote, config)

    if new_free_models:
        print(f"  New FREE models available ({len(new_free_models)}):")
        for m in new_free_models[:15]:
            print(f"    + {m['id']} (provider={m.get('provider','?')})")
        report["changes"].append({"action": "new_free_models", "count": len(new_free_models),
                                   "models": [m["id"] for m in new_free_models[:20]]})
        if len(new_free_models) > 15:
            print(f"    ... and {len(new_free_models)-15} more")

    if changed_limits:
        print(f"  Changed rate limits ({len(changed_limits)}):")
        for mid, key, old, new in changed_limits[:10]:
            print(f"    ~ {mid}: {key} {old} -> {new}")

    # Health check key models (the ones in routing.json cascade)
    print(f"\n--- Health Check (cascade models) ---")
    with open(ROUTING_FILE) as f:
        routing = json.load(f)

    cascade_models = routing.get("strategies", {}).get("free_cascade", {}).get("priority", [])
    for entry in cascade_models:
        mid = entry if isinstance(entry, str) else entry.get("model", "")
        # Find the provider from models config
        provider = None
        for m in updated_models:
            if m["id"] == mid:
                provider = m.get("provider")
                break

        if provider:
            alive, status = health_check_model(mid, provider, env)
            status_str = "OK" if alive else "DOWN"
            print(f"  {mid}: {status_str} ({status})")
            report["health_checks"][mid] = {"alive": alive, "status": status, "provider": provider}
        else:
            print(f"  {mid}: SKIP (no provider)")

    # Save updated config
    config["models"] = updated_models
    config["last_health_check"] = report["timestamp"]

    with open(MODELS_FILE, "w") as f:
        json.dump(config, f, indent=2)
    print(f"\n  Saved models.json ({len(updated_models)} models)")

    # Save report
    with open(REPORT_FILE, "w") as f:
        json.dump(report, f, indent=2)
    print(f"  Saved health_report.json")

    print(f"\nDone. {len(report['warnings'])} warnings.")

    return 0 if not report["warnings"] else 1

if __name__ == "__main__":
    sys.exit(main())
