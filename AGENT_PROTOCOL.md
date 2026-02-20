# Agent Coordination Protocol
# Kimi ↔ GLM-5 Real-Time Collaboration

## The Reality

**Direct terminal notification doesn't work reliably** because:
- OpenCode CLI may intercept terminal output
- Different PTY sessions don't share notifications
- File watching requires active polling

## The Solution: Lightweight Pull Protocol

### For Every Interaction

**Before you start working:**
```bash
chatnew  # Pull + read latest AGENT_CHAT.md
```

**After you post:**
```bash
git add AGENT_CHAT.md
git commit -m "Chat: Your message summary"
git push
# Other agent sees it on their next `chatnew`
```

### The `chatnew` Command

Added to both agents' `.bashrc`:
```bash
alias chatnew='cd ~/vibepilot && git pull && echo "=== AGENT CHAT ===" && tail -30 AGENT_CHAT.md'
alias chat='tail -100 ~/vibepilot/AGENT_CHAT.md'
```

## Why This Works

1. **No polling** - You pull when YOU are ready to work
2. **Always fresh** - `chatnew` gets latest before you act
3. **No missed messages** - Git history has everything
4. **Simple** - No complex infrastructure

## Workflow

```
Kimi wants to respond:
  ↓
Run: chatnew (gets GLM-5's last message)
  ↓
Read + write response
  ↓
git commit + push
  ↓
Done

GLM-5 wants to respond:
  ↓
Run: chatnew (gets Kimi's response)
  ↓
Read + write response
  ↓
git commit + push
  ↓
Done
```

## Auto-Commit Helper

The existing `auto_commit_chat.sh` runs every 2 minutes to prevent data loss.
But for real-time coordination: **always run `chatnew` before working**.

## Emergency: Need Immediate Attention?

If you need the other agent to see something URGENT:

1. Post in AGENT_CHAT.md
2. Commit + push
3. Also post in the **human's chat** asking them to notify the other agent

This is the "escalation path" for critical issues.

## Success Metric

**Goal:** Human never has to say "check chat"

**How:** Both agents habitually run `chatnew` before every work session.
