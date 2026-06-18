#!/usr/bin/env python3
"""
VibePilot System Watchdog
=========================
Runs every 30 minutes via Hermes no-agent cron job.
Checks gateway health and cron job freshness.
SILENT when everything is fine.
Sends email + writes dashboard status when something is wrong.

Exit codes:
  0 = all healthy (no output = silent cron delivery)
  1 = issues found (stdout = alert message for cron delivery)
"""

import json
import os
import smtplib
import subprocess
import sys
from datetime import datetime, timezone, timedelta
from email.mime.text import MIMEText

# ── Config ──────────────────────────────────────────────────────────────────
GMAIL_USER = "vibesagentai@gmail.com"
GMAIL_APP_PASSWORD = "fjzy imce plvj zwdx"
ALERT_RECIPIENT = "vibesagentai@gmail.com"

GATEWAY_URL = "http://127.0.0.1:8642/health"
CRON_JOBS_FILE = os.path.expanduser("~/.hermes/cron/jobs.json")
DB_CONN = "postgres://vibes@/vibepilot?host=/var/run/postgresql"

# ET timezone for "should have run today" logic
ET_OFFSET = -4  # EDT
et_tz = timezone(timedelta(hours=ET_OFFSET))

# Cron jobs to monitor: job_id -> (name, max_age_hours, severity)
# max_age_hours = how long since last run before we flag it
WATCHED_JOBS = {
    "7f8cad8673f8": ("VibePilot Daily Model Health Check", 30, "high"),
    "3b902ab93ef0": ("KB Daily Sync", 30, "high"),
    "12aef300fdf2": ("VibePilot Daily Research Scan", 30, "high"),
    "c2ccabbf89e4": ("Hermes Fallback Auto-Update", 15, "medium"),
    "a315f05eaae5": ("TokenFinder Model Discovery", 15, "medium"),
    "8e2a4b79a688": ("sync-memories", 2, "low"),
    "495b20e4f982": ("jcodemunch-reindex", 8, "low"),
}


def log(msg):
    """Log to stderr so it doesn't pollute cron stdout."""
    print(f"[watchdog] {msg}", file=sys.stderr)


def send_email(subject, body):
    """Send alert email via Gmail SMTP."""
    try:
        msg = MIMEText(body)
        msg["Subject"] = subject
        msg["From"] = f"VibePilot Watchdog <{GMAIL_USER}>"
        msg["To"] = ALERT_RECIPIENT

        with smtplib.SMTP_SSL("smtp.gmail.com", 465, timeout=15) as s:
            s.login(GMAIL_USER, GMAIL_APP_PASSWORD)
            s.send_message(msg)
        log(f"Alert email sent: {subject}")
        return True
    except Exception as e:
        log(f"Failed to send email: {e}")
        return False


def write_dashboard_status(issues):
    """Write current health status to PostgreSQL for dashboard visibility."""
    try:
        # Upsert a single row that the dashboard can read
        status = "HEALTHY" if not issues else "ALERT"
        severity = max((i["severity"] for i in issues), default="info") if issues else "info"

        sql = """
        INSERT INTO system_health (id, status, severity, issues, checked_at, issue_count)
        VALUES (1, %s, %s, %s::jsonb, NOW(), %s)
        ON CONFLICT (id) DO UPDATE SET
            status = EXCLUDED.status,
            severity = EXCLUDED.severity,
            issues = EXCLUDED.issues,
            checked_at = EXCLUDED.checked_at,
            issue_count = EXCLUDED.issue_count;
        """

        issues_json = json.dumps(issues) if issues else "[]"
        count = len(issues)

        import psycopg2
        conn = psycopg2.connect(DB_CONN)
        cur = conn.cursor()
        cur.execute(sql, (status, severity, issues_json, count))
        conn.commit()
        cur.close()
        conn.close()
        log(f"Dashboard status written: {status} ({count} issues)")
        return True
    except Exception as e:
        log(f"Failed to write dashboard status: {e}")
        # Try to create the table if it doesn't exist
        try:
            import psycopg2
            conn = psycopg2.connect(DB_CONN)
            cur = conn.cursor()
            cur.execute("""
                CREATE TABLE IF NOT EXISTS system_health (
                    id INT PRIMARY KEY DEFAULT 1,
                    status TEXT NOT NULL DEFAULT 'UNKNOWN',
                    severity TEXT DEFAULT 'info',
                    issues JSONB DEFAULT '[]'::jsonb,
                    checked_at TIMESTAMPTZ DEFAULT NOW(),
                    issue_count INT DEFAULT 0,
                    CHECK (id = 1)
                );
            """)
            conn.commit()
            cur.close()
            conn.close()
            log("Created system_health table on retry")
            return write_dashboard_status(issues)
        except Exception as e2:
            log(f"Failed to create table: {e2}")
            return False


def check_gateway():
    """Check if the gateway is actually responding."""
    issues = []
    try:
        import urllib.request
        resp = urllib.request.urlopen(GATEWAY_URL, timeout=10)
        if resp.status != 200:
            issues.append({
                "component": "gateway",
                "severity": "critical",
                "message": f"Gateway returned HTTP {resp.status}",
                "last_checked": datetime.now(et_tz).isoformat(),
            })
    except Exception as e:
        issues.append({
            "component": "gateway",
            "severity": "critical",
            "message": f"Gateway not responding: {str(e)[:100]}",
            "last_checked": datetime.now(et_tz).isoformat(),
        })
    return issues


def check_cron_jobs():
    """Check if cron jobs have run recently and succeeded."""
    issues = []

    if not os.path.exists(CRON_JOBS_FILE):
        issues.append({
            "component": "cron_scheduler",
            "severity": "critical",
            "message": "jobs.json not found - cron system may be broken",
            "last_checked": datetime.now(et_tz).isoformat(),
        })
        return issues

    try:
        with open(CRON_JOBS_FILE) as f:
            data = json.load(f)

        jobs = data.get("jobs", data) if isinstance(data, dict) else data
        now = datetime.now(et_tz)

        for job in jobs:
            job_id = job.get("id", "?")
            name = job.get("name", job_id)
            paused = job.get("paused", False)

            # Skip jobs we don't monitor
            if job_id not in WATCHED_JOBS:
                continue

            watched_name, max_age, severity = WATCHED_JOBS[job_id]

            if paused:
                continue  # Don't alert on intentionally paused jobs

            last_status = job.get("last_status", "unknown")
            last_run_str = job.get("last_run_at")

            # Check 1: Job hasn't run recently
            if last_run_str:
                try:
                    last_run = datetime.fromisoformat(last_run_str)
                    if last_run.tzinfo is None:
                        last_run = last_run.replace(tzinfo=timezone.utc)
                    age_hours = (now.astimezone(timezone.utc) - last_run).total_seconds() / 3600

                    if age_hours > max_age:
                        issues.append({
                            "component": f"cron:{watched_name}",
                            "severity": severity,
                            "message": f"Last ran {age_hours:.1f}h ago (expected within {max_age}h)",
                            "last_run": last_run_str,
                            "last_status": last_status,
                        })
                except Exception:
                    pass
            else:
                issues.append({
                    "component": f"cron:{watched_name}",
                    "severity": severity,
                    "message": "Has never run",
                    "last_run": None,
                    "last_status": "never",
                })

            # Check 2: Last run errored
            if last_status == "error" and last_run_str:
                try:
                    last_run = datetime.fromisoformat(last_run_str)
                    if last_run.tzinfo is None:
                        last_run = last_run.replace(tzinfo=timezone.utc)
                    age_hours = (now.astimezone(timezone.utc) - last_run).total_seconds() / 3600
                    # Only alert on errors from the last 2 hours (not ancient history)
                    if age_hours < 2:
                        issues.append({
                            "component": f"cron:{watched_name}",
                            "severity": severity,
                            "message": f"Last run ERRORED ({age_hours:.1f}h ago)",
                            "last_run": last_run_str,
                            "last_status": "error",
                        })
                except Exception:
                    pass

    except Exception as e:
        issues.append({
            "component": "cron_scheduler",
            "severity": "critical",
            "message": f"Failed to parse jobs.json: {str(e)[:100]}",
        })

    return issues


def main():
    log("Starting watchdog check...")

    all_issues = []

    # 1. Gateway health
    all_issues.extend(check_gateway())

    # 2. Cron job freshness + status
    all_issues.extend(check_cron_jobs())

    # 3. Write to dashboard DB (always, so dashboard shows latest check time)
    write_dashboard_status(all_issues)

    # 4. Alert if issues found
    if all_issues:
        # Build alert message
        lines = ["VIBETRIBE SYSTEM ALERT", "=" * 50, ""]

        critical = [i for i in all_issues if i["severity"] == "critical"]
        high = [i for i in all_issues if i["severity"] == "high"]
        medium = [i for i in all_issues if i["severity"] == "medium"]
        low = [i for i in all_issues if i["severity"] == "low"]

        if critical:
            lines.append(f"CRITICAL ({len(critical)}):")
            for i in critical:
                lines.append(f"  [{i['component']}] {i['message']}")
            lines.append("")

        if high:
            lines.append(f"HIGH ({len(high)}):")
            for i in high:
                lines.append(f"  [{i['component']}] {i['message']}")
            lines.append("")

        if medium:
            lines.append(f"MEDIUM ({len(medium)}):")
            for i in medium:
                lines.append(f"  [{i['component']}] {i['message']}")
            lines.append("")

        if low:
            lines.append(f"LOW ({len(low)}):")
            for i in low:
                lines.append(f"  [{i['component']}] {i['message']}")
            lines.append("")

        lines.append("")
        lines.append(f"Checked: {datetime.now(et_tz).strftime('%Y-%m-%d %H:%M %Z')}")
        lines.append("Dashboard: https://vibeflow-dashboard.vercel.app")

        alert_text = "\n".join(lines)

        # Send email
        send_email(f"[VibePilot] {len(all_issues)} system issue(s) detected", alert_text)

        # Output for cron delivery (so it shows up in Hermes too)
        print(alert_text)
        sys.exit(1)
    else:
        # All good - stay silent
        log("All systems healthy. Staying silent.")
        sys.exit(0)


if __name__ == "__main__":
    main()
