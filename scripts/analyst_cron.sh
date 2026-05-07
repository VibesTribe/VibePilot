#!/usr/bin/env bash
# Analyst Proactive — Daily pattern scan + proposal generation
# Runs as a cron job. Scans task data, generates proposals into research_suggestions.
# Reuses existing pipeline: simple proposals auto-implement, complex go to council.
#
# Install: 0 5 * * * /home/vibes/vibepilot/scripts/analyst_cron.sh >> /tmp/analyst.log 2>&1

set -e

LOG="/tmp/analyst-$(date +%Y%m%d).log"
echo "=== Analyst Proactive Scan $(date -Iseconds) ===" >> "$LOG"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"

# Step 1: Run pattern scan
echo "--- Running pattern scan ---" >> "$LOG"
SCAN_OUTPUT=$(python3 "$SCRIPT_DIR/analyst_pattern_scan.py" --hours 24 --json 2>>"$LOG")
echo "$SCAN_OUTPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'  Found {len(d)} findings')" >> "$LOG"

# Step 2: Insert proposals into research_suggestions
echo "--- Inserting proposals ---" >> "$LOG"
INSERTED=0
echo "$SCAN_OUTPUT" | python3 -c "
import sys, json
findings = json.load(sys.stdin)
for f in findings:
    conf = f.get('confidence', 0)
    if conf < 0.6:
        print(f'  DEFERRED (conf={conf:.2f}): {f[\"proposed_action\"][:60]}')
        continue

    complexity = 'simple' if conf >= 0.8 else 'complex'

    import subprocess
    title = f['proposed_action']
    summary = f['evidence']
    details = json.dumps({
        'evidence': f['evidence'],
        'impact': f['impact'],
        'confidence': conf,
        'source': 'analyst_pattern_scan v1',
        'raw_source': f.get('source', '')
    })

    sql = f\"\"\"INSERT INTO research_suggestions
    (type, title, summary, complexity, details, findings_path, status, source)
    VALUES ('analyst_proposal', {json.dumps(title)}, {json.dumps(summary)},
            '{complexity}', '{details}'::jsonb,
            'scripts/analyst_pattern_scan.py', 'pending', 'analyst_proactive')
    ON CONFLICT DO NOTHING;\"\"\"

    r = subprocess.run(['psql', '-d', 'vibepilot', '-c', sql],
                       capture_output=True, text=True, timeout=10)
    if r.returncode == 0:
        print(f'  INSERTED ({complexity}, conf={conf:.2f}): {title[:60]}')
    else:
        print(f'  ERROR: {r.stderr[:100]}')
" >> "$LOG" 2>&1

echo "=== Done $(date -Iseconds) ===" >> "$LOG"
