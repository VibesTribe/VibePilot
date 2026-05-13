#!/usr/bin/env python3
"""Aggregate Hermes agent session usage into models table. Check all thresholds and send alerts."""
import psycopg, os, smtplib
from email.message import EmailMessage

DB_URL = os.environ.get("DATABASE_URL", "dbname=vibepilot host=/var/run/postgresql")
GMAIL_PASS = os.environ.get("VIBEPILOT_GMAIL_PASSWORD", "")
FROM_EMAIL = "vibesagentai@gmail.com"
ALERT_EMAIL = "capreol30@gmail.com"
ALERTED_MODELS = set()

def send_alert(subject, body):
    if not GMAIL_PASS:
        print("  [Alert] No GMAIL_PASSWORD set, skipping email")
        return
    try:
        msg = EmailMessage()
        msg["From"] = FROM_EMAIL
        msg["To"] = ALERT_EMAIL
        msg["Subject"] = f"[!] VibePilot: {subject}"
        msg.set_content(body)
        with smtplib.SMTP("smtp.gmail.com", 587) as s:
            s.starttls()
            s.login(FROM_EMAIL, GMAIL_PASS)
            s.send_message(msg)
        print(f"  [Alert] Sent: {subject}")
    except Exception as e:
        print(f"  [Alert] Failed: {e}")

conn = psycopg.connect(DB_URL)
cur = conn.cursor()

# Step 1: Aggregate agent session totals into models table
cur.execute("""
    SELECT model_id, SUM(total_tokens), SUM(total_tokens_in),
           SUM(total_tokens_out), SUM(total_cost_usd), COUNT(*), SUM(message_count)
    FROM agent_sessions WHERE model_id IS NOT NULL AND status = 'active'
    GROUP BY model_id
""")
for row in cur.fetchall():
    mid, tokens, ti, to, cost, sessions, msgs = row
    conn.execute("UPDATE models SET token_used = GREATEST(COALESCE(token_used,0), %s) WHERE id = %s",
                 (tokens, mid))
    print(f"  {mid}: {tokens:,} tokens synced")

# Step 2: Check ALL threshold types across all models
cur.execute("""
    SELECT id, credit_total_usd, credit_remaining_usd, credit_alert_threshold,
           request_limit, request_used, subscription_status, status, status_reason
    FROM models
""")
for row in cur.fetchall():
    mid, c_total, c_rem, c_thresh, r_limit, r_used, sub_status, status, s_reason = row
    
    alerts = []
    needs_pause = False
    pause_reason = None
    
    # Credit check (DeepSeek-style)
    if c_total and c_total > 0 and c_rem is not None:
        ratio = c_rem / c_total
        thresh = c_thresh if c_thresh else 0.8
        if ratio <= 0:
            needs_pause = True
            pause_reason = "Credit exhausted"
            alerts.append(f"CRITICAL: {mid} credit depleted (${c_rem:.2f})")
        elif ratio < 1.0 - thresh:
            alerts.append(f"Credit at {ratio*100:.0f}% (${c_rem:.2f} of ${c_total:.2f})")
    
    # Request limit check (GLM-style subscriptions)
    if r_limit and r_limit > 0 and r_used is not None:
        req_ratio = r_used / r_limit
        if req_ratio >= 1.0:
            needs_pause = True
            pause_reason = f"Request limit reached ({r_used}/{r_limit})"
            alerts.append(f"CRITICAL: {mid} request limit exhausted ({r_used}/{r_limit})")
        elif req_ratio >= 0.8:
            alerts.append(f"Requests at {req_ratio*100:.0f}% ({r_used}/{r_limit})")
    
    # Auto-pause models that are exhausted  
    if needs_pause and status == 'active':
        conn.execute("UPDATE models SET status = 'paused', status_reason = %s WHERE id = %s",
                     (pause_reason, mid))
        print(f"  [!] PAUSED {mid}: {pause_reason}")
    
    # Send alert if new
    if alerts and mid not in ALERTED_MODELS:
        ALERTED_MODELS.add(mid)
        for a in alerts:
            send_alert(f"{mid}: {a.split(':')[0]}", 
                       f"Model: {mid}\nAlert: {a}\nStatus: {'PAUSED' if needs_pause else 'active'}\n\n"
                       f"Dashboard: https://vibeflow-dashboard.vercel.app")

# Step 3: Ensure Gemini 3.1 Flash Lite Preview migration reminder
cur.execute("SELECT id FROM model_catalog WHERE id = 'gemini-3.1-flash-lite-preview' AND status = 'active'")
if cur.fetchone():
    print("  [!] REMINDER: gemini-3.1-flash-lite-preview still active - will be discontinued May 25")

conn.commit()
conn.close()
print("Done")
