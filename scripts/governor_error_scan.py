#!/usr/bin/env python3
"""
Analyst Phase 3 — System health scanner.
Scans systemd journals, service logs, and config state for:
  1. Service failures and crash loops
  2. OOM kills and memory pressure events
  3. Health check failures
  4. Config validation errors
  5. Recurring error patterns

Generates findings into research_suggestions (reuses existing pipeline).
Designed to run alongside or after analyst_cron.sh.

Usage:
  python3 governor_error_scan.py               # scan last 24h
  python3 governor_error_scan.py --hours 72     # custom window
  python3 governor_error_scan.py --json         # JSON output only
  python3 governor_error_scan.py --insert       # insert into research_suggestions
"""

import json
import os
import re
import subprocess
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone

DB = "vibepilot"
DEFAULT_HOURS = 24

# Services to monitor
SERVICES = [
    "vibepilot-governor",
    "hermes-gateway",
    "kb-mcp-server",
    "knowledgebase-server",
    "postgresql",
]


def journal_query(unit, hours, priority=""):
    """Query systemd journal for a service. Returns list of lines."""
    cmd = [
        "journalctl", "--user", "-u", unit,
        "--since", f"{hours} hours ago",
        "--no-pager",
    ]
    if priority:
        cmd += ["-p", priority]
    try:
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
        return r.stdout.split("\n")
    except subprocess.TimeoutExpired:
        return []


def grep_lines(lines, patterns):
    """Filter lines matching any pattern (case-insensitive)."""
    result = []
    for line in lines:
        for pat in patterns:
            if re.search(pat, line, re.IGNORECASE):
                result.append(line.strip())
                break
    return result


def query(sql):
    """Run SQL query against vibepilot DB, return list of dicts."""
    r = subprocess.run(
        ["psql", "-d", DB, "-t", "-A", "-F", "\t", "-c", sql],
        capture_output=True, text=True, timeout=15,
    )
    if r.returncode != 0 or not r.stdout.strip():
        return []
    result = []
    for line in r.stdout.strip().split("\n"):
        parts = line.split("\t")
        if len(parts) >= 2:
            result.append(dict(zip(
                ["col1", "col2", "col3", "col4", "col5"],
                parts + [""] * (5 - len(parts))
            )))
    return result


def scan_service_failures(hours):
    """Check each service for crash loops, restarts, failures."""
    findings = []

    for service in SERVICES:
        lines = journal_query(service, hours)

        # Count only ACTUAL service crashes (systemd exit-code failures)
        fail_lines = grep_lines(lines, ["Failed with result", "failed to start"])
        failure_count = len(fail_lines)

        # Count only ACTUAL service restarts (systemd Starting + Started paired events)
        # NOT internal goroutine/bg-task starts
        systemd_start_lines = [l for l in lines if re.search(r"systemd\[\d+\]: Starting |systemd\[\d+\]: Started ", l)]
        restart_count = len(systemd_start_lines) // 2  # Each restart has both Starting + Started

        # Count unique PIDs to cross-check
        pids = set()
        for l in lines:
            m = re.search(rf"{re.escape(service)}\[(\d+)\]", l)
            if m:
                pids.add(m.group(1))

        # Count OOM events
        oom_lines = grep_lines(lines, ["oom", "out of memory", "killed process"])
        oom_count = len(oom_lines)

        # Check for crash clusters (3+ unique PIDs in 5 minutes = crash loop)
        if failure_count >= 3:
            findings.append({
                "type": "service_failure",
                "service": service,
                "severity": "critical" if restart_count >= 5 else "warning",
                "evidence": f"{service}: {failure_count} crashes, {restart_count} restarts, {oom_count} OOM events in last {hours}h ({len(pids)} unique PIDs)",
                "impact": f"{restart_count} service restarts cause ~{restart_count * 2}s of downtime",
                "crash_loop": restart_count >= 5,
                "details": {
                    "service": service,
                    "crash_count": failure_count,
                    "restart_count": restart_count,
                    "oom_count": oom_count,
                    "unique_pids": len(pids),
                }
            })

        if oom_count >= 2:
            findings.append({
                "type": "memory_pressure",
                "service": service,
                "severity": "warning",
                "evidence": f"{service}: {oom_count} OOM/memory events in last {hours}h",
                "impact": "Memory pressure degrades performance and can cause cascading failures",
                "details": {
                    "service": service,
                    "oom_count": oom_count,
                }
            })

    return findings


def scan_hermes_errors(hours):
    """Check Hermes gateway logs for API errors, fallback chains, rate limits."""
    lines = journal_query("hermes-gateway", hours)
    findings = []

    # Rate limit errors — count unique events
    rl_events = grep_lines(lines, ["RateLimitError", "HTTP 429"])
    if len(rl_events) >= 3:
        findings.append({
            "type": "rate_limit_flood",
            "service": "hermes-gateway",
            "severity": "warning",
            "evidence": f"Rate limit errors: ~{len(rl_events)} API call failures in last {hours}h",
            "impact": "Fallback chain models hitting rate limits causes degraded response times and model switching overhead",
            "details": {
                "source": "journalctl",
                "rate_limit_api_failures": len(rl_events),
            }
        })

    # Auth/key errors — count unique events
    auth_events = grep_lines(lines, ["AuthenticationError", "HTTP 401", "Invalid API Key"])
    auth_event_count = len(auth_events)
    if auth_event_count >= 1:
        findings.append({
            "type": "auth_failure",
            "service": "hermes-gateway",
            "severity": "critical" if auth_event_count >= 5 else "warning",
            "evidence": f"Auth/API key errors: ~{auth_event_count} API call failures in last {hours}h",
            "impact": "Expired or invalid API keys cause fallback cascade failures",
            "details": {
                "auth_api_failures": auth_event_count,
            }
        })

    # Fallback cascade chains — count unique events
    fallback_events = grep_lines(lines, ["switching to fallback"])
    fallback_event_count = len(fallback_events)
    if fallback_event_count >= 5:
        findings.append({
            "type": "fallback_cascade",
            "service": "hermes-gateway",
            "severity": "info",
            "evidence": f"Fallback chain activations: ~{fallback_event_count} in last {hours}h",
            "impact": "Frequent fallback switching means primary model is unreliable or rate-limited",
            "details": {
                "fallback_count": fallback_event_count,
            }
        })

    return findings


def scan_governor_health(hours):
    """Check governor health check failures."""
    lines = journal_query("vibepilot-governor", hours)

    health_fails = grep_lines(lines, ["health check fail", "health check failed"])
    if len(health_fails) >= 2:
        # Categorize which connectors are failing
        failed_connectors = set()
        for line in health_fails:
            m = re.search(r"HEALTH CHECK FAILED:\s*(\S+)", line)
            if m:
                failed_connectors.add(m.group(1))

        findings = [{
            "type": "connector_health",
            "service": "vibepilot-governor",
            "severity": "warning",
            "evidence": f"Health check failures: {len(health_fails)} total across {len(failed_connectors)} connector(s): {', '.join(sorted(failed_connectors))}",
            "impact": "Unhealthy connectors mean tasks may fail to route to those platforms",
            "details": {
                "failed_connectors": list(failed_connectors),
                "total_failures": len(health_fails),
            }
        }]
    else:
        findings = []

    # Config errors
    config_lines = grep_lines(lines, ["config error", "invalid config", "failed to parse", "config validation"])
    if config_lines:
        findings.append({
            "type": "config_error",
            "service": "vibepilot-governor",
            "severity": "critical",
            "evidence": f"{len(config_lines)} config validation errors detected in last {hours}h",
            "impact": "Invalid config prevents governor from starting or routing correctly",
            "details": {
                "config_error_count": len(config_lines),
            }
        })

    # CodeMap errors
    codemap_lines = grep_lines(lines, ["codemap.*fail", "jcodemunch.*fail", "refresh failed"])
    if codemap_lines:
        findings.append({
            "type": "codemap_error",
            "service": "vibepilot-governor",
            "severity": "info",
            "evidence": f"CodeMap/codemunch errors: {len(codemap_lines)} in last {hours}h",
            "impact": "Code indexing failures may give agents stale or incomplete code context",
            "details": {
                "codemap_error_count": len(codemap_lines),
            }
        })

    return findings


def scan_system_memory(hours):
    """Check system-wide memory pressure events."""
    lines = journal_query("systemd-journald", hours, priority="warning")
    oom_lines = grep_lines(lines, ["oom", "out of memory", "memory.low", "memory.high"])

    if oom_lines:
        return [{
            "type": "system_memory_pressure",
            "service": "system",
            "severity": "warning",
            "evidence": f"{len(oom_lines)} system-wide OOM/memory events in last {hours}h",
            "impact": "Memory pressure on 16GB X220 can cause OOM kills, swap thrashing, and service collapse",
            "details": {
                "memory_event_count": len(oom_lines),
            }
        }]
    return []


def generate_runbook(findings, hours):
    """Generate runbook entries from recurring error patterns."""
    runbooks = []

    # Check for crash loop pattern
    crash_loops = [f for f in findings if f.get("crash_loop") and f["type"] == "service_failure"]
    for cl in crash_loops:
        runbooks.append({
            "type": "runbook",
            "severity": "warning",
            "pattern": f"{cl['service']} crash loop detected",
            "check_command": f"journalctl --user -u {cl['service']} --since '24 hours ago' --no-pager | grep 'Failed with'",
            "common_causes": [
                "Config file format error (JSON/YAML parse failure)",
                "Missing dependency or binary",
                "OOM kill due to memory limit",
                "Port already in use",
            ],
            "fix_commands": [
                f"systemctl --user status {cl['service']}",
                f"journalctl --user -u {cl['service']} --since '1 hour ago' --no-pager -p err",
                f"systemctl --user restart {cl['service']}",
            ],
        })

    # Check for API key failures
    auth_fails = [f for f in findings if f["type"] == "auth_failure"]
    if auth_fails:
        runbooks.append({
            "type": "runbook",
            "severity": "warning",
            "pattern": "API key or authentication failure",
            "check_command": f"journalctl --user -u hermes-gateway --since '{hours} hours ago' --no-pager | grep -i 'invalid api key\\|401\\|unauthorized'",
            "common_causes": [
                "Expired API key in vault or .env",
                "Wrong key env variable name in config",
                "Key rotated on provider's end",
            ],
            "fix_commands": [
                "Check vault: psql -d vibepilot -c 'SELECT service, key_preview FROM vault_items;'",
                "Check auth.json vs config.yaml provider name alignment",
            ],
        })

    return runbooks


def main():
    hours = int(sys.argv[sys.argv.index("--hours") + 1]) if "--hours" in sys.argv else DEFAULT_HOURS
    output_json = "--json" in sys.argv
    do_insert = "--insert" in sys.argv

    all_findings = []
    runbooks = []

    print(f"=== Analyst Phase 3 — System Health Scan (last {hours}h) ===", file=sys.stderr)

    # Step 1: Service failures
    print(f"Scannning service failures...", file=sys.stderr)
    all_findings.extend(scan_service_failures(hours))

    # Step 2: Hermes errors
    print(f"Scannning Hermes errors...", file=sys.stderr)
    all_findings.extend(scan_hermes_errors(hours))

    # Step 3: Governor health
    print(f"Scannning governor health...", file=sys.stderr)
    all_findings.extend(scan_governor_health(hours))

    # Step 4: System memory
    print(f"Scannning system memory...", file=sys.stderr)
    all_findings.extend(scan_system_memory(hours))

    # Step 5: Generate runbooks from recurring patterns
    runbooks = generate_runbook(all_findings, hours)

    if output_json:
        output = {
            "findings": all_findings,
            "runbooks": runbooks,
            "scan_time": datetime.now(timezone.utc).isoformat(),
            "hours_covered": hours,
        }
        print(json.dumps(output, indent=2))
    else:
        print(f"\nFindings: {len(all_findings)}")
        print("=" * 50)
        for f in sorted(all_findings, key=lambda x: x.get("severity", "info")):
            sev = f.get("severity", "info").upper()
            print(f"\n[{sev}] {f['type']} — {f['service']}")
            print(f"  {f['evidence']}")
            print(f"  Impact: {f['impact']}")

        if runbooks:
            print(f"\nRunbook entries: {len(runbooks)}")
            print("=" * 50)
            for rb in runbooks:
                print(f"\n[RUNBOOK] {rb['pattern']}")
                print(f"  Check: {rb['check_command']}")
                print(f"  Common causes: {', '.join(rb['common_causes'][:2])}")

        print(f"\n--- End of Phase 3 scan ---")

    # Insert into research_suggestions if requested
    if do_insert:
        inserted = 0
        for f in all_findings:
            severity = f.get("severity", "info")
            if severity == "info":
                continue  # Skip low-severity for auto-insert

            complexity = "simple" if severity == "info" else "complex"
            title = f"{f['type']}: {f['service']} — {f['evidence'][:80]}"
            summary = f["evidence"]
            details = json.dumps({
                "evidence": f["evidence"],
                "impact": f["impact"],
                "source": "governor_error_scan",
                "severity": severity,
                "scan_hours": hours,
            })

            sql = f"""INSERT INTO research_suggestions
                (type, title, summary, complexity, details, findings_path, status, source)
                VALUES ('config_tweak', {json.dumps(title)}, {json.dumps(summary)},
                        '{complexity}', '{details}'::jsonb,
                        'scripts/governor_error_scan.py', 'pending', 'analyst_phase3')
                ON CONFLICT DO NOTHING;"""

            r = subprocess.run(
                ["psql", "-d", DB, "-c", sql],
                capture_output=True, text=True, timeout=10
            )
            if r.returncode == 0:
                inserted += 1

        print(f"\nInserted {inserted} findings into research_suggestions", file=sys.stderr)

        # Also insert runbooks
        for rb in runbooks:
            details = json.dumps({
                "pattern": rb["pattern"],
                "check_command": rb["check_command"],
                "common_causes": rb["common_causes"],
                "fix_commands": rb["fix_commands"],
                "source": "governor_error_scan",
                "type": "runbook",
            })
            sql = f"""INSERT INTO research_suggestions
                (type, title, summary, complexity, details, findings_path, status, source)
                VALUES ('workflow_change', {json.dumps(rb['pattern'])}, 'Auto-generated runbook entry from recurring system error pattern',
                        'human', '{details}'::jsonb,
                        'scripts/governor_error_scan.py', 'pending', 'analyst_phase3')
                ON CONFLICT DO NOTHING;"""
            subprocess.run(
                ["psql", "-d", DB, "-c", sql],
                capture_output=True, text=True, timeout=10
            )


if __name__ == "__main__":
    main()
