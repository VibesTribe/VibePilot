#!/usr/bin/env python3
"""
Start listening for agent messages in background.
Usage: python scripts/listen_for_messages.py glm-5

This starts a background thread that polls for new messages
and prints notifications when they arrive.
"""

import sys
import os
import time
import signal

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from dotenv import load_dotenv

load_dotenv()

from core.agent_comm import AgentComm

AGENT = sys.argv[1] if len(sys.argv) > 1 else "glm-5"


def handle_message(msg):
    from_agent = msg.get("from_agent", "unknown")
    content = msg.get("content", {})
    text = content.get("text", str(content))[:500]
    msg_type = msg.get("message_type", "chat")

    print(f"\n{'!' * 60}")
    print(f"📨 NEW MESSAGE from {from_agent} [{msg_type}]")
    print(f"{'!' * 60}")
    print(text)
    print(f"{'!' * 60}\n")


comm = AgentComm(AGENT, on_message=handle_message)


def signal_handler(sig, frame):
    print(f"\n[AgentComm] Stopping listener for {AGENT}...")
    comm.stop()
    sys.exit(0)


signal.signal(signal.SIGINT, signal_handler)

print(f"[AgentComm] Starting listener for {AGENT}...")
print(f"[AgentComm] Press Ctrl+C to stop")
print(f"[AgentComm] Listening...\n")

comm.start_listening(poll_mode=True)

# Keep running
while True:
    time.sleep(1)
