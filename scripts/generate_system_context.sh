#!/bin/bash
# Generates a live system context snapshot for researcher and council agents.
# Output: JSON with all current system state (versions, models, connectors, health).
# Called by governor on startup and before each research run.

set -euo pipefail

# Build JSON using python for proper formatting (bash json is fragile)
python3 -c '
import json, subprocess, os, re

def run(cmd):
    try:
        r = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=10)
        return r.stdout.strip()
    except:
        return "unknown"

def run_int(cmd):
    v = run(cmd)
    try: return int(v)
    except: return 0

def run_float(cmd):
    v = run(cmd)
    try: return float(v)
    except: return 0.0

data = {
    "generated_at": run("date -u +%Y-%m-%dT%H:%M:%SZ"),
    "generated_by": "generate_system_context.sh",

    "hardware": {
        "machine": "ThinkPad X220",
        "cpu": run("grep \"model name\" /proc/cpuinfo | head -1 | cut -d: -f2 | xargs"),
        "has_avx2": "avx2" in run("grep -o \"avx2\" /proc/cpuinfo | head -1").lower(),
        "has_gpu": False,
        "ram_total_gb": run_int("free -g | awk \"/^Mem:/{print \\$2}\""),
        "ram_available_gb": run_int("free -g | awk \"/^Mem:/{print \\$7}\""),
        "disk_used_pct": run_int("df / | awk \"NR==2{print \\$5}\" | tr -d %"),
        "load_1min": run_float("cut -d\" \" -f1 /proc/loadavg"),
    },

    "software": {
        "go": run("go version 2>/dev/null | awk \"{print \\$3}\""),
        "node": run("node --version 2>/dev/null"),
        "npm": run("npm --version 2>/dev/null"),
        "python": run("python3 --version 2>/dev/null | awk \"{print \\$2}\""),
        "postgres": run("psql --version 2>/dev/null | awk \"{print \\$3}\""),
        "git": run("git --version 2>/dev/null | awk \"{print \\$3}\""),
        "hermes": run("hermes version 2>/dev/null | head -1 | awk \"{print \\$NF}\""),
        "cloudflared": run("cloudflared --version 2>/dev/null | awk \"{print \\$3}\""),
        "playwright": run("npx playwright --version 2>/dev/null"),
        "vercel_cli": run("vercel --version 2>/dev/null | head -1"),
    },

    "go_deps": {
        "pgx": run("grep \"jackc/pgx\" /home/vibes/vibepilot/governor/go.mod 2>/dev/null | awk \"{print \\$2}\""),
        "mcp_go": run("grep \"mark3labs/mcp-go\" /home/vibes/vibepilot/governor/go.mod 2>/dev/null | awk \"{print \\$2}\""),
    },

    "services": {
        "governor": run("pgrep -f \"vibepilot/governor/governor\" > /dev/null && echo true || echo false") == "true",
        "hermes": run("pgrep -f \"hermes\" > /dev/null && echo true || echo false") == "true",
        "cloudflared": run("pgrep -f \"cloudflared\" > /dev/null && echo true || echo false") == "true",
        "postgres": run("pg_isready > /dev/null 2>&1 && echo true || echo false") == "true",
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
try:
    ext_raw = run("""psql -d vibepilot -t -A -c "SELECT extname, extversion FROM pg_extension WHERE extname IN (\""vector\"",\""pg_trgm\"",\""btree_gin\"");" 2>/dev/null""")
    exts = []
    for line in ext_raw.split("\n"):
        if "|" in line:
            parts = line.split("|")
            exts.append({"name": parts[0].strip(), "version": parts[1].strip()})
    data["postgres_extensions"] = exts
except:
    data["postgres_extensions"] = []

# Model catalog summary
try:
    total = run_int("psql -d vibepilot -t -A -c 'SELECT COUNT(*) FROM model_catalog;' 2>/dev/null")
    active = run_int("psql -d vibepilot -t -A -c \"SELECT COUNT(*) FROM model_catalog WHERE status='active';\" 2>/dev/null")
    benched = run_int("psql -d vibepilot -t -A -c \"SELECT COUNT(*) FROM model_catalog WHERE status='benched';\" 2>/dev/null")
    data["model_catalog"] = {"total": total, "active": active, "benched": benched}
except:
    data["model_catalog"] = {"total": 0, "active": 0, "benched": 0}

# Active connectors
try:
    conn_raw = run("psql -d vibepilot -t -A -c \"SELECT id, type, status FROM connectors WHERE status='active';\" 2>/dev/null")
    conns = []
    for line in conn_raw.split("\n"):
        if "|" in line:
            parts = line.split("|")
            conns.append({"id": parts[0].strip(), "type": parts[1].strip(), "status": parts[2].strip()})
    data["connectors"] = conns
except:
    data["connectors"] = []

# Model health last 24h
try:
    health_raw = run("psql -d vibepilot -t -A -c \"SELECT model_id, role, SUM(CASE WHEN success THEN 1 ELSE 0 END) as ok, SUM(CASE WHEN NOT success THEN 1 ELSE 0 END) as fail, ROUND(100.0*SUM(CASE WHEN success THEN 1 ELSE 0 END)/NULLIF(COUNT(*),0),1) as rate FROM model_usage WHERE created_at > NOW()-INTERVAL '24 hours' GROUP BY model_id, role ORDER BY COUNT(*) DESC LIMIT 10;\" 2>/dev/null")
    health = []
    for line in health_raw.split("\n"):
        if "|" in line:
            parts = line.split("|")
            health.append({"model": parts[0].strip(), "role": parts[1].strip(), "successes": int(parts[2].strip()), "failures": int(parts[3].strip()), "rate": parts[4].strip()+"%"})
    data["model_health_24h"] = health
except:
    data["model_health_24h"] = []

# Research pipeline
try:
    pending = run_int("psql -d vibepilot -t -A -c \"SELECT COUNT(*) FROM research_suggestions WHERE status='pending';\" 2>/dev/null")
    council = run_int("psql -d vibepilot -t -A -c \"SELECT COUNT(*) FROM research_suggestions WHERE status='council_review';\" 2>/dev/null")
    human = run_int("psql -d vibepilot -t -A -c \"SELECT COUNT(*) FROM research_reports WHERE status='pending_human';\" 2>/dev/null")
    review = run_int("psql -d vibepilot -t -A -c \"SELECT COUNT(*) FROM review_items WHERE status='pending';\" 2>/dev/null")
    data["research_pipeline"] = {"pending_suggestions": pending, "council_review": council, "pending_human": human, "review_items": review}
except:
    data["research_pipeline"] = {"pending_suggestions": 0, "council_review": 0, "pending_human": 0, "review_items": 0}

print(json.dumps(data, indent=2))
'
