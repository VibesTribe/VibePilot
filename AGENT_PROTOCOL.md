# Agent Communication Protocol

**Mandatory reading at EVERY session start.**

This is the single source of truth for how agents coordinate.

---

## Golden Rule

> **Check AGENT_CHAT.md FIRST. Always. Every session.**

No exceptions. No "I'll check later." 

**Before ANY work:**
1. `git pull origin main`
2. `cat AGENT_CHAT.md`
3. `./check_chat.sh --once` (mark as read)
4. Respond to any messages waiting

---

## Communication Channels (Simplified)

### PRIMARY: AGENT_CHAT.md
**Use for:** Real-time coordination, questions, status updates

**When to use:**
- "I'm working on X"
- "I need your input on Y"
- "Status update: Z is done"
- "Quick question..."
- Any back-and-forth

**Check:** At session start, and before ending session

### SECONDARY: inbox/{agent_name}/
**Use for:** Research assignments, task handoffs, detailed findings

**When to use:**
- "Research this and report back"
- "Here's a complex analysis"
- "Task assignment with full context"
- "Files too big for chat"

**Check:** At session start (after AGENT_CHAT.md)

**Format:** Use header:
```markdown
---
from: glm-5
 to: kimi
type: task|response|fyi
priority: high|medium|low
created: 2026-02-18T21:45:00Z
---
```

### TERTIARY: .handoff-to-{agent}.md
**Use for:** Session summaries, complex state transfers

**When to use:**
- Ending session with work in progress
- Complex context that needs preservation
- State that next session must know

**Check:** At session start (if exists and newer than last session)

---

## Quick Reference

| If you want to... | Use | Example |
|-------------------|-----|---------|
| Ask a quick question | AGENT_CHAT.md | "What do you think about X?" |
| Say "I'm working on Y" | AGENT_CHAT.md | "Starting on token tracking" |
| Request research | inbox/{name}/ | "Research JSONB migration options" |
| Report findings | inbox/{name}/ | Detailed analysis with code |
| End session with WIP | .handoff-to-{name}.md | "Task 50% done, here's state" |
| Share status | AGENT_CHAT.md | "Finished Z, committed to main" |

---

## Anti-Patterns (Don't Do This)

❌ **Put message in random file** - "They'll find it somewhere"
❌ **Assume they know** - "They probably saw my commit"
❌ **Multiple channels for same thing** - Chat + inbox + handoff for one question
❌ **Not checking at start** - Starting work without seeing if someone needs you
❌ **Session without closing loop** - Disappearing without updating status

---

## Session Start Checklist

**Copy-paste this every session:**

```bash
# 1. Sync
cd ~/vibepilot
git checkout main
git pull origin main

# 2. Check primary communication
cat AGENT_CHAT.md
./check_chat.sh --once

# 3. Check secondary (if exists)
ls inbox/kimi/ 2>/dev/null && cat inbox/kimi/*

# 4. Check handoff (if exists and new)
if [ -f .handoff-to-kimi.md ]; then
    stat .handoff-to-kimi.md
    # If newer than your last session, read it
fi

# 5. NOW start work
```

---

## Session End Checklist

**Before you finish:**

```bash
# 1. Update chat
# Add message to AGENT_CHAT.md with status

# 2. If work in progress
# Create .handoff-to-{other}.md with state

# 3. Commit everything
git add -A
git commit -m "Status: [what you did]"
git push origin main

# 4. Mark chat read
./check_chat.sh --once
```

---

## Real-Time Teamwork

**What this enables:**

- **Parallel work:** Both agents active, checking chat every few minutes
- **Quick questions:** "Should I do X or Y?" → Response in minutes
- **No lost messages:** Single protocol, mandatory check
- **Clear handoffs:** Who's doing what, when

**Response time expectation:**
- Chat messages: Check at session start + every 15 min
- Inbox tasks: Respond same session if possible
- Handoffs: Read immediately, acknowledge

---

## Emergency Escalation

If you need immediate response:

1. Post in AGENT_CHAT.md with **URGENT** tag
2. Also post brief note in .handoff-to-{name}.md
3. Human will coordinate if needed

**Don't abuse.** Use for blockers only.

---

## This Session

**Kimi:** Checked AGENT_CHAT.md ✅  
**GLM:** [Update when you see this]  

**Outstanding items:**
- JSONB migration: Kimi approved ✅ (GLM implementing)
- Token calculator: Ready for wiring
- Council practice: Both preparing

**Next sync point:** When GLM sees this, reply in chat

---

**Remember:** We are a team. Teams communicate. Check the chat.
