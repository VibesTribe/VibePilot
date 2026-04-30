#!/usr/bin/env python3
"""
VibePilot Agent Gmail Integration

Manages the dedicated Gmail account for VibePilot agents to:
- Sign up for free tier web AI platforms
- Receive access codes and verification emails
- Send daily digests and alerts to human

Usage:
    # Check for verification emails from platforms
    python scripts/research/gmail_agent.py --check-verification
    
    # Send daily digest
    python scripts/research/gmail_agent.py --send-digest --to human@email.com
    
    # Check all inbox and categorize
    python scripts/research/gmail_agent.py --inbox
"""

import os
import sys
import json
import base64
import argparse
from datetime import datetime, timedelta
from pathlib import Path
from typing import List, Dict, Optional

sys.path.insert(0, str(Path(__file__).parent.parent.parent))

# Google API imports
try:
    from google.auth.transport.requests import Request
    from google.oauth2.credentials import Credentials
    from google_auth_oauthlib.flow import InstalledAppFlow
    from googleapiclient.discovery import build
    from googleapiclient.errors import HttpError
except ImportError:
    print("Google API client not installed. Run: pip install google-auth google-auth-oauthlib google-auth-httplib2 google-api-python-client")
    sys.exit(1)

from dotenv import load_dotenv
load_dotenv()

# Gmail API Scopes
SCOPES = [
    'https://www.googleapis.com/auth/gmail.readonly',
    'https://www.googleapis.com/auth/gmail.send',
    'https://www.googleapis.com/auth/gmail.modify'  # For marking as read/archiving
]

# Token storage
TOKEN_FILE = Path(__file__).parent / '.gmail_token.json'
CREDENTIALS_FILE = Path(__file__).parent / '.gmail_credentials.json'


class GmailAgent:
    """Gmail interface for VibePilot agent operations."""
    
    def __init__(self):
        self.service = None
        self._authenticate()
    
    def _authenticate(self):
        """Authenticate with Gmail API."""
        creds = None
        
        # Load existing token
        if TOKEN_FILE.exists():
            creds = Credentials.from_authorized_user_file(str(TOKEN_FILE), SCOPES)
        
        # If no valid credentials, run auth flow
        if not creds or not creds.valid:
            if creds and creds.expired and creds.refresh_token:
                creds.refresh(Request())
            else:
                if not CREDENTIALS_FILE.exists():
                    print("Gmail credentials not found.")
                    print(f"Place credentials JSON at: {CREDENTIALS_FILE}")
                    print("\nTo get credentials:")
                    print("  1. Go to https://console.cloud.google.com/")
                    print("  2. Create project or select existing")
                    print("  3. Enable Gmail API")
                    print("  4. Create OAuth 2.0 credentials (Desktop app)")
                    print("  5. Download JSON and save as .gmail_credentials.json")
                    sys.exit(1)
                
                flow = InstalledAppFlow.from_client_secrets_file(
                    str(CREDENTIALS_FILE), SCOPES)
                creds = flow.run_local_server(port=0)
            
            # Save token for future runs
            with open(TOKEN_FILE, 'w') as token:
                token.write(creds.to_json())
        
        self.service = build('gmail', 'v1', credentials=creds)
        print("✓ Gmail authenticated")
    
    def get_profile(self) -> Dict:
        """Get Gmail profile info."""
        return self.service.users().getProfile(userId='me').execute()
    
    def check_verification_emails(self) -> List[Dict]:
        """
        Check for verification/access code emails from AI platforms.
        Returns list of verification emails.
        """
        # Search for common verification patterns
        queries = [
            'subject:(verification OR verify OR code OR confirm) newer_than:7d',
            'subject:(access OR activation OR welcome) newer_than:7d',
            'from:(noreply OR no-reply OR support) newer_than:7d'
        ]
        
        verifications = []
        
        for query in queries:
            results = self.service.users().messages().list(
                userId='me',
                q=query,
                maxResults=20
            ).execute()
            
            messages = results.get('messages', [])
            
            for msg in messages:
                email_data = self._get_message_details(msg['id'])
                if email_data:
                    # Extract potential codes
                    codes = self._extract_codes(email_data['body'])
                    if codes:
                        email_data['extracted_codes'] = codes
                        verifications.append(email_data)
                    elif 'verification' in email_data['subject'].lower() or 'verify' in email_data['subject'].lower():
                        verifications.append(email_data)
        
        return verifications
    
    def _get_message_details(self, msg_id: str) -> Optional[Dict]:
        """Get full message details."""
        try:
            message = self.service.users().messages().get(
                userId='me', id=msg_id, format='full'
            ).execute()
            
            headers = message['payload']['headers']
            subject = next((h['value'] for h in headers if h['name'] == 'Subject'), 'No Subject')
            sender = next((h['value'] for h in headers if h['name'] == 'From'), 'Unknown')
            date = next((h['value'] for h in headers if h['name'] == 'Date'), '')
            
            # Get body
            body = self._get_message_body(message['payload'])
            
            return {
                'id': msg_id,
                'subject': subject,
                'sender': sender,
                'date': date,
                'body': body[:2000]  # First 2000 chars
            }
            
        except Exception as e:
            print(f"Error fetching message {msg_id}: {e}")
            return None
    
    def _get_message_body(self, payload) -> str:
        """Extract text body from message payload."""
        body = ""
        
        if 'parts' in payload:
            for part in payload['parts']:
                if part['mimeType'] == 'text/plain':
                    data = part['body'].get('data', '')
                    if data:
                        body += base64.urlsafe_b64decode(data).decode('utf-8', errors='ignore')
                elif part['mimeType'] == 'text/html':
                    # Fallback to HTML if no plain text
                    if not body:
                        data = part['body'].get('data', '')
                        if data:
                            html = base64.urlsafe_b64decode(data).decode('utf-8', errors='ignore')
                            # Simple HTML to text
                            import re
                            body = re.sub(r'<[^>]+>', ' ', html)
                elif 'parts' in part:
                    body += self._get_message_body(part)
        else:
            data = payload['body'].get('data', '')
            if data:
                body = base64.urlsafe_b64decode(data).decode('utf-8', errors='ignore')
        
        return body
    
    def _extract_codes(self, text: str) -> List[str]:
        """Extract verification codes from text."""
        import re
        
        # Common code patterns
        patterns = [
            r'\b\d{6}\b',  # 6-digit codes
            r'code[\s:]+([A-Z0-9]{4,8})',
            r'code[\s:]+(\d{4,8})',
            r'verification[\s:]+([A-Z0-9-]{4,12})',
        ]
        
        codes = []
        for pattern in patterns:
            matches = re.findall(pattern, text, re.IGNORECASE)
            codes.extend(matches)
        
        return list(set(codes))[:5]  # Max 5 unique codes
    
    def send_digest(self, to_email: str, subject: str, content: str) -> bool:
        """Send daily digest email."""
        try:
            message = self._create_message(to_email, subject, content)
            self.service.users().messages().send(userId='me', body=message).execute()
            print(f"✓ Digest sent to {to_email}")
            return True
        except Exception as e:
            print(f"Error sending email: {e}")
            return False
    
    def _create_message(self, to: str, subject: str, body: str) -> Dict:
        """Create email message."""
        from email.mime.text import MIMEText
        
        profile = self.get_profile()
        from_email = profile['emailAddress']
        
        message = MIMEText(body, 'plain', 'utf-8')
        message['to'] = to
        message['from'] = from_email
        message['subject'] = subject
        
        raw = base64.urlsafe_b64encode(message.as_bytes()).decode('utf-8')
        return {'raw': raw}
    
    def list_labels(self) -> List[Dict]:
        """List Gmail labels."""
        results = self.service.users().labels().list(userId='me').execute()
        return results.get('labels', [])
    
    def check_inbox(self, max_results: int = 20) -> List[Dict]:
        """Check inbox for recent emails."""
        results = self.service.users().messages().list(
            userId='me',
            labelIds=['INBOX'],
            maxResults=max_results
        ).execute()
        
        messages = []
        for msg in results.get('messages', []):
            details = self._get_message_details(msg['id'])
            if details:
                messages.append(details)
        
        return messages
    
    def archive_processed(self, msg_ids: List[str]):
        """Archive processed emails."""
        for msg_id in msg_ids:
            try:
                self.service.users().messages().modify(
                    userId='me',
                    id=msg_id,
                    body={'removeLabelIds': ['INBOX']}
                ).execute()
            except Exception as e:
                print(f"Error archiving {msg_id}: {e}")


def generate_daily_digest() -> str:
    """Generate daily digest content for human."""
    date_str = datetime.now().strftime('%Y-%m-%d %H:%M UTC')
    
    digest = f"""VibePilot Daily Digest - {date_str}
{'='*50}

SYSTEM STATUS
-----------
- Orchestrator: Running
- Active models: kimi-cli, glm-5 (opencode)
- Courier platforms: Active (web platforms)
- Tasks in queue: Check dashboard

RESEARCH FINDINGS
-----------------
"""
    
    # This would be populated from actual research results
    digest += "Run research scripts to populate this section.\n\n"
    
    digest += """VERIFICATION CODES
------------------
Check with: python scripts/research/gmail_agent.py --check-verification

PLATFORM STATUS
---------------
- Gemini API: Quota exhausted (resets midnight PT)
- DeepSeek API: Credit needed
- Kimi CLI: Active ($0.99 promo, 7 days left)
- OpenCode (GLM): Active

RECOMMENDATIONS
---------------
1. Review high-priority research items
2. Check dashboard for task status
3. Monitor rate limits before heavy usage

---
Reply to this email for questions.
VibePilot Agent Gmail
"""
    
    return digest


def main():
    parser = argparse.ArgumentParser(description='VibePilot Gmail Agent')
    parser.add_argument('--check-verification', action='store_true', 
                        help='Check for verification/access code emails')
    parser.add_argument('--send-digest', action='store_true',
                        help='Send daily digest')
    parser.add_argument('--to', help='Recipient email for digest')
    parser.add_argument('--inbox', action='store_true',
                        help='List recent inbox emails')
    parser.add_argument('--labels', action='store_true',
                        help='List Gmail labels')
    
    args = parser.parse_args()
    
    # Initialize Gmail agent
    agent = GmailAgent()
    
    if args.labels:
        print("Gmail Labels:")
        for label in agent.list_labels():
            print(f"  - {label['name']} ({label['id']})")
    
    elif args.check_verification:
        print("Checking for verification emails...")
        verifications = agent.check_verification_emails()
        
        if verifications:
            print(f"\nFound {len(verifications)} verification emails:")
            for v in verifications:
                print(f"\nFrom: {v['sender']}")
                print(f"Subject: {v['subject']}")
                if 'extracted_codes' in v:
                    print(f"Codes: {', '.join(v['extracted_codes'])}")
                print(f"Date: {v['date']}")
        else:
            print("No verification emails found in last 7 days.")
    
    elif args.send_digest:
        if not args.to:
            print("Error: --to required for sending digest")
            sys.exit(1)
        
        digest = generate_daily_digest()
        date_str = datetime.now().strftime('%Y-%m-%d')
        subject = f"VibePilot Daily Digest - {date_str}"
        
        agent.send_digest(args.to, subject, digest)
    
    elif args.inbox:
        print("Recent inbox emails:")
        emails = agent.check_inbox()
        for e in emails:
            print(f"\nFrom: {e['sender']}")
            print(f"Subject: {e['subject']}")
            print(f"Preview: {e['body'][:100]}...")
    
    else:
        # Show profile
        profile = agent.get_profile()
        print(f"Gmail Agent Profile:")
        print(f"  Email: {profile['emailAddress']}")
        print(f"  Messages: {profile['messagesTotal']}")
        print(f"  Threads: {profile['threadsTotal']}")
        print(f"\nUse --help for available commands")


if __name__ == '__main__':
    main()
