#!/usr/bin/env python3
"""Send email notifications for VibePilot alerts.

Called by the governor credit poller with --subject and --body.
Uses Gmail SMTP credentials from config file.

Usage:
    python3 notify_email.py --subject "Subject" --body "Body text" [--to email@example.com]
"""
import argparse
import json
import os
import smtplib
import sys
from email.mime.text import MIMEText

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
CREDS_FILE = os.path.join(SCRIPT_DIR, ".email_creds.json")
DEFAULT_TO = "capreol30@gmail.com"


def load_creds():
    """Load email credentials from config file."""
    if not os.path.exists(CREDS_FILE):
        print(f"ERROR: Credentials file not found: {CREDS_FILE}", file=sys.stderr)
        sys.exit(1)
    
    with open(CREDS_FILE, "r") as f:
        creds = json.load(f)
    
    required = ["email", "password"]
    for key in required:
        if key not in creds:
            print(f"ERROR: Missing '{key}' in credentials file", file=sys.stderr)
            sys.exit(1)
    
    return creds


def send_email(subject, body, to_email, creds):
    """Send email via Gmail SMTP."""
    msg = MIMEText(body)
    msg['Subject'] = subject
    msg['From'] = creds["email"]
    msg['To'] = to_email

    smtp_host = creds.get("smtp_host", "smtp.gmail.com")
    smtp_port = creds.get("smtp_port", 465)

    with smtplib.SMTP_SSL(smtp_host, smtp_port) as server:
        server.login(creds["email"], creds["password"])
        server.send_message(msg)
    return True


def main():
    parser = argparse.ArgumentParser(description="Send notification email")
    parser.add_argument("--subject", required=True, help="Email subject")
    parser.add_argument("--body", required=True, help="Email body")
    parser.add_argument("--to", default=DEFAULT_TO, help="Recipient email")
    args = parser.parse_args()

    creds = load_creds()
    
    try:
        send_email(args.subject, args.body, args.to, creds)
        print(f"Email sent to {args.to}: {args.subject}")
    except Exception as e:
        print(f"ERROR: Failed to send email: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
