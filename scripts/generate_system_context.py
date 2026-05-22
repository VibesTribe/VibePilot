#!/usr/bin/env python3
"""Generates a live system context snapshot for researcher and council agents.
Output: JSON with all current system state (versions, models, connectors, health).
Called by governor on startup and before each research run.
"""

import json, subprocess, os, re
from datetime import datetime, timezone

def run(cmd):
    try:
        r = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=10)
        return r.stdout.strip()
    except:
        return ""

def run_int(cmd):
    v = run(cmd)
    try: return int(v)
    except: return 0

def run_float(cmd):
    v = run(cmd)
    try: return float(v)
    except: return 0.0

def psql(query):
    """Run a psql query and return stripped output."""
    return run(f"psql -d vibepilot -t -A -c \"{query}\" 2>/dev/null")

def psql_int(query):
    v = psql(query)
    try: return int(v.split("\n")[0].strip())
    except: return 0

def psql_rows(query):
    raw = psql(query)
    rows = []
    for line in raw.split("\n"):
        if "|" in line:
            rows.append([p.strip() for p in line.split("|")])
    return rows

now = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

data = {
    "generated_at": now,
    "generated_by": "generate_system_context.py",

    "hardware": {
        "machine": "ThinkPad X220",
        "cpu": run("grep 'model name' /proc/cpuinfo | head -1 | cut -d: -f2 | xargs"),
        "has_avx2": "avx2" in run("grep '^flags' /proc/cpuinfo | head -1"),
        "has_gpu": False,
        "ram_total_gb": run_int("free -g | awk '/^Mem:/{print $2}'"),
        "ram_available_gb": run_int("free -g | awk '/^Mem:/{print $7}'"),
        "disk_used_pct": run_int("df / | awk 'NR==2{print $5}' | tr -d %"),
        "load_1min": run_float("cut -d' ' -f1 /proc/loadavg"),
    },

    "software": {
        "go": run("go version 2>/dev/null | awk '{print $3}'"),
        "node": run("node --version 2>/dev/null"),
        "npm": run("npm --version 2>/dev/null"),
        "python": run("python3 --version 2>/dev/null | awk '{print $2}'"),
        "postgres": run("psql --version 2>/dev/null | awk '{print $3}'"),
        "git": run("git --version 2>/dev/null | awk '{print $3}'"),
        "hermes": run("hermes version 2>/dev/null | head -1"),
        "cloudflared": run("cloudflared --version 2>/dev/null | awk '{print $3}'"),
        "playwright": run("npx playwright --version 2>/dev/null"),
        "vercel_cli": run("vercel --version 2>/dev/null | head -1"),
    },

    "go_deps": {
        "pgx": run("grep 'jackc/pgx' /home/vibes/vibepilot/governor/go.mod 2>/dev/null | awk '{print $2}'"),
        "mcp_go": run("grep 'mark3labs/mcp-go' /home/vibes/vibepilot/governor/go.mod 2>/dev/null | awk '{print $2}'"),
    },

    "services": {
        "governor": bool(run("pgrep -f 'vibepilot/governor/governor'")),
        "hermes": bool(run("pgrep -f 'hermes'")),
        "cloudflared": bool(run("pgrep -f 'cloudflared'")),
        "postgres": run("pg_isready > /dev/null 2>&1 && echo yes || echo") == "yes",
    },

    "constraints": {
        "cost": "$0/month target, free-tier-only, user is unemployed",
        "location": "Toronto, Canada - many US-only features unavailable",
        "hardware_limits": "No GPU, no AVX2, no Docker, no local models, 16GB RAM, spinning HDD",
        "licensing": "MIT or Apache 2.0 for open-source, generous free tiers for SaaS",
        "no_paid_apis": True,
        "no_local_inference": True,
        "no_docker": True,
    },
}

# Postgres extensions
exts = []
for row in psql_rows("SELECT extname, extversion FROM pg_extension WHERE extname IN ('vector','pg_trgm','btree_gin');"):
    exts.append({"name": row[0], "version": row[1]})
data["postgres_extensions"] = exts

# Model catalog summary
data["model_catalog"] = {
    "total": psql_int("SELECT COUNT(*) FROM model_catalog;"),
    "active": psql_int("SELECT COUNT(*) FROM model_catalog WHERE status='active';"),
    "benched": psql_int("SELECT COUNT(*) FROM model_catalog WHERE status='benched';"),
}

# Active connectors
conns = []
for row in psql_rows("SELECT id, type, status FROM connectors WHERE status='active';"):
    conns.append({"id": row[0], "type": row[1], "status": row[2]})
data["connectors"] = conns

# Model health last 24h
health = []
for row in psql_rows(
    "SELECT model_id, role, SUM(CASE WHEN success THEN 1 ELSE 0 END), "
    "SUM(CASE WHEN NOT success THEN 1 ELSE 0 END), "
    "ROUND(100.0*SUM(CASE WHEN success THEN 1 ELSE 0 END)/NULLIF(COUNT(*),0),1) "
    "FROM model_usage WHERE created_at > NOW()-INTERVAL '24 hours' "
    "GROUP BY model_id, role ORDER BY COUNT(*) DESC LIMIT 10;"
):
    health.append({"model": row[0], "role": row[1], "successes": int(row[2]), "failures": int(row[3]), "rate": row[4]+"%"})
data["model_health_24h"] = health

# Research pipeline state
data["research_pipeline"] = {
    "pending_suggestions": psql_int("SELECT COUNT(*) FROM research_suggestions WHERE status='pending';"),
    "council_review": psql_int("SELECT COUNT(*) FROM research_suggestions WHERE status='council_review';"),
    "pending_human": psql_int("SELECT COUNT(*) FROM research_reports WHERE status='pending_human';"),
    "review_items": psql_int("SELECT COUNT(*) FROM review_items WHERE status='pending';"),
}

print(json.dumps(data, indent=2))
