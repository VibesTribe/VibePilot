#!/usr/bin/env python3
"""Send credit alert email notifications using vault-encrypted Gmail credentials.

Uses the same AES-GCM decryption as the Go vault (PBKDF2 + AES-256-GCM).
Called by the governor credit alert system or manually.

Usage:
    python3 credit_alert_email.py [--dry-run]
"""
import base64
import hashlib
import os
import smtplib
import subprocess
import sys
from email.mime.text import MIMEText

import psycopg2
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
from cryptography.hazmat.primitives import hashes

# Vault constants (must match Go vault.go)
SALT_SIZE = 16
NONCE_SIZE = 12
PBKDF2_ITERATIONS = 100000
KEY_SIZE = 32

ALERT_EMAIL = "capreol30@gmail.com"


def get_vault_key():
    """Get VAULT_KEY from the running governor process environment."""
    try:
        pid = subprocess.check_output(
            ["pgrep", "-f", "governor/governor"], text=True
        ).strip().split('\n')[0]
        environ = subprocess.check_output(
            ["cat", f"/proc/{pid}/environ"], text=False
        )
        for entry in environ.split(b'\0'):
            if entry.startswith(b'VAULT_KEY='):
                return entry.decode().split('=', 1)[1]
    except Exception:
        pass
    # Fallback to env var
    return os.environ.get('VAULT_KEY', '')


def derive_key(password: str, salt: bytes) -> bytes:
    """PBKDF2 key derivation matching Go's deriveKey."""
    kdf = PBKDF2HMAC(
        algorithm=hashes.SHA256(),
        length=KEY_SIZE,
        salt=salt,
        iterations=PBKDF2_ITERATIONS,
    )
    return kdf.derive(password.encode())


def decrypt(encrypted_b64: str, master_key: str) -> str:
    """AES-GCM decryption matching Go vault.decrypt."""
    ciphertext = base64.b64decode(encrypted_b64)
    if len(ciphertext) < SALT_SIZE + NONCE_SIZE + 1:
        raise ValueError("ciphertext too short")

    salt = ciphertext[:SALT_SIZE]
    nonce = ciphertext[SALT_SIZE:SALT_SIZE + NONCE_SIZE]
    actual_ct = ciphertext[SALT_SIZE + NONCE_SIZE:]

    key = derive_key(master_key, salt)
    aesgcm = AESGCM(key)
    plaintext = aesgcm.decrypt(nonce, actual_ct, None)
    return plaintext.decode()


def get_secret(key_name: str, vault_key: str) -> str:
    """Decrypt a secret from the secrets_vault table."""
    conn = psycopg2.connect("dbname=vibepilot host=/var/run/postgresql")
    cur = conn.cursor()
    cur.execute(
        "SELECT encrypted_value FROM secrets_vault WHERE key_name = %s",
        (key_name,)
    )
    row = cur.fetchone()
    cur.close()
    conn.close()
    if not row:
        raise ValueError(f"No vault entry for {key_name}")
    return decrypt(row[0], vault_key)


def get_credit_alerts():
    """Get current credit alerts from the DB function."""
    conn = psycopg2.connect("dbname=vibepilot host=/var/run/postgresql")
    cur = conn.cursor()
    cur.execute("SELECT * FROM check_subscription_thresholds()")
    alerts = cur.fetchall()
    cur.close()
    conn.close()
    return alerts


def send_email(subject, body, gmail_email, gmail_password, dry_run=False):
    """Send email via Gmail SMTP."""
    if dry_run:
        print(f"[DRY RUN] To: {ALERT_EMAIL}")
        print(f"[DRY RUN] Subject: {subject}")
        print(f"[DRY RUN] Body:\n{body}")
        return True

    msg = MIMEText(body)
    msg['Subject'] = subject
    msg['From'] = gmail_email
    msg['To'] = ALERT_EMAIL

    with smtplib.SMTP_SSL('smtp.gmail.com', 465) as server:
        server.login(gmail_email, gmail_password)
        server.send_message(msg)
    return True


def main():
    dry_run = '--dry-run' in sys.argv

    alerts = get_credit_alerts()
    if not alerts:
        print("No credit alerts. Nothing to send.")
        return

    vault_key = get_vault_key()
    if not vault_key:
        print("ERROR: Could not find VAULT_KEY")
        sys.exit(1)

    gmail_email = get_secret('VIBEPILOT_GMAIL_EMAIL', vault_key)
    gmail_password = get_secret('VIBEPILOT_GMAIL_PASSWORD', vault_key)

    lines = []
    for row in alerts:
        model_id, alert_type, current_val, threshold_val, message = row
        lines.append(f"{message}")
        lines.append(f"  Current: ${current_val:.2f} | Threshold: ${threshold_val:.2f}")
        lines.append("")

    subject = f"VibePilot Credit Alert: {len(alerts)} model(s) below threshold"
    body = (
        "The following model(s) have credit balances below their alert thresholds:\n\n"
        + "\n".join(lines)
        + "\nReview at: https://vibeflow-dashboard.vercel.app/admin#credits\n"
    )

    send_email(subject, body, gmail_email, gmail_password, dry_run)
    if not dry_run:
        print(f"Sent credit alert email for {len(alerts)} alert(s)")


if __name__ == '__main__':
    main()
