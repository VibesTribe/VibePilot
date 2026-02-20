#!/usr/bin/env python3
"""Quick check for agent messages. Usage: python scripts/check_agent_mail.py [agent_name]"""
import sys
import os
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from supabase import create_client
from dotenv import load_dotenv

load_dotenv()

from vault_manager import get_api_key

AGENT = sys.argv[1] if len(sys.argv) > 1 else "kimi"

url = os.getenv("SUPABASE_URL")
key = get_api_key("SUPABASE_SERVICE_KEY")
client = create_client(url, key)

# Get unread
result = client.table("agent_messages").select("*").eq("to_agent", AGENT).is_("read_at", "null").execute()

if not result.data:
    print(f"📬 No unread messages for {AGENT}")
else:
    print(f"📬 {len(result.data)} UNREAD for {AGENT}:")
    print("="*60)
    for msg in result.data:
        print(f"\nFrom: {msg['from_agent']}")
        print(f"Type: {msg['message_type']}")
        print(f"Content:")
        content = msg['content']
        if isinstance(content, dict):
            print(content.get('text', str(content)))
        else:
            print(content)
        print("-"*60)
