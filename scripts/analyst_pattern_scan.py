#!/usr/bin/env python3
"""
Analyst Proactive — pattern scan query.
Queries task_runs and tasks for cross-task failure patterns.
Outputs structured findings for the analyst_proactive.md prompt.

Usage:
  python3 analyst_pattern_scan.py          # scan last 24h
  python3 analyst_pattern_scan.py --hours 72  # custom window
  python3 analyst_pattern_scan.py --json   # JSON output only
"""

import json
import os
import sys
import subprocess
from datetime import datetime, timezone
from collections import defaultdict

DB = "vibepilot"
DEFAULT_HOURS = 24

def query(sql):
    r = subprocess.run(
        ["psql", "-d", DB, "-t", "-A", "-F", "|", "-c", sql],
        capture_output=True, text=True, timeout=30
    )
    if r.returncode != 0:
        print(f"SQL error: {r.stderr}", file=sys.stderr)
        return []
    lines = [l.strip() for l in r.stdout.split("\n") if l.strip()]
    return lines

def main():
    hours = int(sys.argv[sys.argv.index("--hours") + 1]) if "--hours" in sys.argv else DEFAULT_HOURS
    output_json = "--json" in sys.argv

    findings = []

    # --- Pattern 1: Models with high failure rates per task type ---
    sql1 = f"""
    SELECT tr.model_id, t.type AS task_type,
           COUNT(*) AS total_runs,
           SUM(CASE WHEN tr.status = 'failed' THEN 1 ELSE 0 END) AS failures,
           ROUND(100.0 * SUM(CASE WHEN tr.status = 'failed' THEN 1 ELSE 0 END) / NULLIF(COUNT(*), 0), 1) AS fail_pct,
           STRING_AGG(DISTINCT LEFT(COALESCE(t.failure_notes, ''), 80), ' | ') AS sample_notes
    FROM task_runs tr
    JOIN tasks t ON t.id = tr.task_id
    WHERE tr.started_at > NOW() - INTERVAL '{hours} hours'
      AND tr.model_id IS NOT NULL
    GROUP BY tr.model_id, t.type
    HAVING COUNT(*) >= 3
       AND SUM(CASE WHEN tr.status = 'failed' THEN 1 ELSE 0 END) >= 2
    ORDER BY fail_pct DESC;
    """
    for line in query(sql1):
        parts = line.split("|")
        if len(parts) >= 6:
            findings.append({
                "type": "model_exclusion",
                "evidence": f"{parts[0]} fails on {parts[3]} of {parts[2]} {parts[1]} tasks ({parts[4]}%)",
                "impact": f"Would save ~{int(parts[3])} retries in last {hours}h",
                "proposed_action": f"Exclude {parts[0]} from {parts[1]} category routing",
                "confidence": min(0.5 + float(parts[4]) / 200, 0.95),
                "source": f"Model {parts[0]} on {parts[1]} tasks: {parts[3]}/{parts[2]} failed ({parts[4]}%)"
            })

    # --- Pattern 2: Task categories with high retry counts ---
    sql2 = f"""
    SELECT t.type AS task_type,
           COUNT(*) AS total_tasks,
           ROUND(AVG(t.attempts), 1) AS avg_attempts,
           COUNT(*) FILTER (WHERE t.attempts >= 3) AS high_retry_count
    FROM tasks t
    WHERE t.updated_at > NOW() - INTERVAL '{hours} hours'
      AND t.attempts > 0
    GROUP BY t.type
    HAVING AVG(t.attempts) >= 2.0
    ORDER BY avg_attempts DESC;
    """
    for line in query(sql2):
        parts = line.split("|")
        if len(parts) >= 4:
            findings.append({
                "type": "task_split",
                "evidence": f"{parts[0]} tasks average {parts[2]} attempts ({parts[3]} tasks had 3+)",
                "impact": f"{parts[3]} tasks consuming ~{int(parts[3])*3}x expected retry budget",
                "proposed_action": f"Review {parts[0]} task decomposition — may need smaller units",
                "confidence": min(0.3 + float(parts[2]) / 10, 0.85),
                "source": f"{parts[0]} tasks: avg {parts[2]} attempts, {parts[3]} high-retry"
            })

    # --- Pattern 3: Recurring failure notes ---
    sql3 = f"""
    SELECT LEFT(t.failure_notes, 120) AS note_snippet,
           COUNT(*) AS frequency,
           STRING_AGG(DISTINCT t.type, ', ') AS task_types
    FROM tasks t
    WHERE t.failure_notes IS NOT NULL
      AND t.failure_notes != ''
      AND t.updated_at > NOW() - INTERVAL '{hours} hours'
    GROUP BY LEFT(t.failure_notes, 120)
    HAVING COUNT(*) >= 2
    ORDER BY frequency DESC
    LIMIT 10;
    """
    for line in query(sql3):
        parts = line.split("|")
        if len(parts) >= 3:
            findings.append({
                "type": "prompt_tweak",
                "evidence": f"Recurring failure note ({parts[1]}x): \"{parts[0][:80]}...\"",
                "impact": f"Affects {parts[1]} tasks across {parts[2]}",
                "proposed_action": f"Address failure pattern in prompt: {parts[0][:80]}",
                "confidence": 0.6,
                "source": f"Failure note repeated {parts[1]}x in {parts[2]}"
            })

    # --- Pattern 4: Models excluded across multiple tasks ---
    sql4 = f"""
    SELECT tr.model_id,
           COUNT(DISTINCT tr.task_id) AS tasks_affected,
           COUNT(*) AS total_failures,
           STRING_AGG(DISTINCT t.type, ', ') AS task_types
    FROM task_runs tr
    JOIN tasks t ON t.id = tr.task_id
    WHERE tr.status = 'failed'
      AND tr.started_at > NOW() - INTERVAL '{hours} hours'
    GROUP BY tr.model_id
    HAVING COUNT(DISTINCT tr.task_id) >= 2
    ORDER BY tasks_affected DESC
    LIMIT 10;
    """
    for line in query(sql4):
        parts = line.split("|")
        if len(parts) >= 4:
            findings.append({
                "type": "routing_rule",
                "evidence": f"{parts[0]} failed on {parts[1]} different tasks ({parts[2]} total failures)",
                "impact": f"Wasted {parts[2]} attempts across {parts[3]}",
                "proposed_action": f"Downgrade {parts[0]} routing priority or exclude from {parts[3]}",
                "confidence": min(0.5 + int(parts[1]) / 10, 0.9),
                "source": f"{parts[0]}: {parts[2]} failures across {parts[1]} tasks ({parts[3]})"
            })

    if output_json:
        print(json.dumps(findings, indent=2))
    else:
        print(f"Analyst Proactive Scan — Last {hours}h")
        print(f"Findings: {len(findings)}")
        print("=" * 50)
        for f in findings:
            print(f"\n[{f['type']}] (confidence: {f['confidence']:.2f})")
            print(f"  Evidence: {f['evidence']}")
            print(f"  Impact:   {f['impact']}")
            print(f"  Action:   {f['proposed_action']}")
        print(f"\n--- End of scan ---")

    return findings

if __name__ == "__main__":
    main()
