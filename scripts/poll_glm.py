#!/usr/bin/env python3
"""
Poll for GLM-5 messages in AGENT_CHAT.md
Usage: python3 poll_glm.py [interval_seconds]
Default: 30 seconds
"""

import sys
import os
import time
import re
from datetime import datetime

CHAT_FILE = "/home/mjlockboxsocial/vibepilot/AGENT_CHAT.md"
LAST_CHECK_FILE = "/tmp/last_glm_check_timestamp"
SEEN_MESSAGES_FILE = "/tmp/glm_seen_messages"

def get_last_check_time():
    """Get the last time we checked for messages."""
    if os.path.exists(LAST_CHECK_FILE):
        try:
            with open(LAST_CHECK_FILE, 'r') as f:
                return float(f.read().strip())
        except:
            pass
    return 0

def update_last_check_time():
    """Update the last check timestamp."""
    with open(LAST_CHECK_FILE, 'w') as f:
        f.write(str(time.time()))

def get_file_mtime():
    """Get the modification time of the chat file."""
    try:
        return os.path.getmtime(CHAT_FILE)
    except:
        return 0

def get_seen_message_ids():
    """Get set of message IDs we've already seen."""
    if not os.path.exists(SEEN_MESSAGES_FILE):
        return set()
    try:
        with open(SEEN_MESSAGES_FILE, 'r') as f:
            return set(line.strip() for line in f if line.strip())
    except:
        return set()

def add_seen_message_id(msg_id):
    """Mark a message as seen."""
    with open(SEEN_MESSAGES_FILE, 'a') as f:
        f.write(f"{msg_id}\n")

def find_new_glm_messages():
    """Find new GLM-5 messages since last check."""
    try:
        with open(CHAT_FILE, 'r') as f:
            content = f.read()
    except:
        return []
    
    # Pattern: ### GLM-5 [timestamp] - Title
    pattern = r'###\s+(GLM-5)\s+\[([^\]]+)\]\s+-\s+(.+?)(?=\n\n|\n---|\Z)'
    matches = list(re.finditer(pattern, content, re.MULTILINE | re.DOTALL))
    
    seen = get_seen_message_ids()
    new_messages = []
    
    for match in matches:
        msg_id = f"{match.group(1)}-{match.group(2)}"
        if msg_id not in seen:
            new_messages.append({
                'agent': match.group(1),
                'timestamp': match.group(2),
                'title': match.group(3).strip(),
                'id': msg_id,
                'start_pos': match.start()
            })
            add_seen_message_id(msg_id)
    
    return new_messages

def get_message_content(start_pos, content):
    """Get the full content of a message."""
    try:
        with open(CHAT_FILE, 'r') as f:
            f.seek(start_pos)
            # Read until next header or end of file
            lines = []
            for line in f:
                if line.startswith('### ') and lines:
                    break
                lines.append(line)
                if line.strip() == '---':
                    break
            return ''.join(lines)
    except:
        return "[Could not read message content]"

def alert_new_messages(messages):
    """Alert the user about new messages."""
    if not messages:
        return
    
    print("\n" + "=" * 70)
    print("🚨 NEW GLM-5 MESSAGE(S) DETECTED! 🚨")
    print("=" * 70)
    
    for msg in messages:
        print(f"\n📩 From: {msg['agent']}")
        print(f"🕐 Time: {msg['timestamp']}")
        print(f"📝 Title: {msg['title']}")
    
    print("\n" + "=" * 70)
    print(f"Total new messages: {len(messages)}")
    print(f"Check AGENT_CHAT.md or run: tail -100 {CHAT_FILE}")
    print("=" * 70 + "\n")
    
    # Also log to file
    with open("/tmp/glm_poll.log", 'a') as f:
        f.write(f"{datetime.now()}: Alerted {len(messages)} new GLM-5 message(s)\n")

def show_status():
    """Show current system status."""
    os.chdir("/home/mjlockboxsocial/vibepilot")
    
    # Get git status
    try:
        import subprocess
        last_commit = subprocess.check_output(
            ["git", "log", "--oneline", "-1"],
            text=True
        ).strip().split()[0]
        
        uncommitted = len(subprocess.check_output(
            ["git", "status", "--porcelain"],
            text=True
        ).strip().split('\n')) if subprocess.check_output(
            ["git", "status", "--porcelain"],
            text=True
        ).strip() else 0
        
        print(f"[{datetime.now().strftime('%H:%M:%S')}] "
              f"Last commit: {last_commit} | Uncommitted: {uncommitted} | "
              f"Watching for GLM-5...")
    except:
        print(f"[{datetime.now().strftime('%H:%M:%S')}] Watching for GLM-5...")

def main():
    interval = int(sys.argv[1]) if len(sys.argv) > 1 else 30
    
    print(f"🤖 GLM-5 Message Polling Started")
    print(f"   File: {CHAT_FILE}")
    print(f"   Interval: {interval}s")
    print(f"   Log: /tmp/glm_poll.log")
    print(f"   Press Ctrl+C to stop")
    print("")
    
    # Initialize seen messages from existing file
    if not os.path.exists(SEEN_MESSAGES_FILE):
        print("Initializing seen messages from existing chat...")
        find_new_glm_messages()  # This will mark existing as seen
        print("Ready! Watching for NEW messages only.\n")
    
    counter = 0
    try:
        while True:
            messages = find_new_glm_messages()
            
            if messages:
                alert_new_messages(messages)
            
            # Show status every 10 checks
            counter += 1
            if counter >= 10:
                show_status()
                counter = 0
            
            update_last_check_time()
            time.sleep(interval)
            
    except KeyboardInterrupt:
        print("\n\n👋 Polling stopped.")
        print(f"Log saved to: /tmp/glm_poll.log")

if __name__ == "__main__":
    main()
