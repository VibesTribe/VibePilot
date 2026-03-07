# VibePilot Agent Workflow Guide

**For ALL agents/LLMs working on VibePilot**

---

## ⛔ STOP. READ THIS FIRST. ⛔

### The 2 Files You MUST Read (In Order)

**1.** [VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md](VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md)
   - What VibePilot is and why it exists
   - Core principles (no hardcoding, no Type 1 errors)
   - How the vault works (credentials access)
   - How the dashboard works (READ-ONLY)
   - Complete flow (PRD → task → completion)
   - Deep dive links to detailed docs

**2.** [CURRENT_STATE.md](CURRENT_STATE.md)
   - What's done
   - What's broken
   - What's next
   - Link to CHANGELOG.md for history

### Only Then Act

Now you understand:
- The system architecture
- Current state
- What needs doing
- Where files are

---

## ⛔ ABSOLUTE RULES

### ⛔ NO MULTIPLE CHOICE FORMS

**Never use restrictive form-style multiple choice questions.**

The user hates them. Previous sessions tried this. It was bad.

**Do this instead:**
- Ask open questions naturally in plain text
- Present options in conversation, not as forms
- Let the user respond in their own words

---

## CORE ARCHITECTURE PRINCIPLE

**VibePilot is designed to be 100% swappable, portable, and vendor-agnostic.**

| Component | Can Be Swapped For | How |
|-----------|-------------------|-----|
| **Database** | Supabase → PostgreSQL → MySQL → SQLite → MongoDB | JSONB everywhere, no TEXT[] or UUID[] |
| **Code Host** | GitHub → GitLab → Bitbucket | Git-based, no API lock-in |
| **AI CLI** | OpenCode → Claude CLI → Gemini CLI → Anything | Config-driven destinations |
| **Hosting** | GCP → AWS → Azure → Local | Single binary, config files |
| **Models** | Any LLM with any provider | Routing config, model profiles |

**Implications for ALL code:**

1. **JSONB for all array/object data** - Works in any database, understood by any LLM
2. **Config over code** - Behavior changes = config edit, not code change
3. **No vendor-specific features** - TEXT[] is PostgreSQL-only → use JSONB
4. **Pass slices directly to RPCs** - Go's JSON encoder handles it, no pre-marshaling
5. **All schema changes in `docs/supabase-schema/`** - Human applies from GitHub (source of truth)

**The test:** Can we swap [X] by changing config only? If no, refactor.

---

## Quick Branch Reference

| Branch | Use For | Who Can Push | Approval Needed |
|--------|---------|--------------|-----------------|
| `main` | Backend code, scripts, configs, docs (rollbackable) | Any agent | No (if rollbackable) |
| `research-considerations` | Research findings, analysis, considerations | System Researcher | No (docs only) |
| `feature/*` | UI/dashboard changes, major features | Any agent | **YES - Human must approve** |
| `hotfix/*` | Emergency fixes | Any agent | After fix (retroactive) |

---

## ⚠️ CRITICAL: All Changes Must Be Committed to GitHub

**Files created or modified locally are NOT deployed until pushed to GitHub.**

- The human only sees what's in GitHub
- Migrations, code changes, configs - all must be committed AND pushed
- After making changes: `git add . && git commit -m "message" && git push origin <branch>`

**Supabase Schema Migrations:**
- ALL schema changes go in `docs/supabase-schema/` on GitHub main
- Human copies SQL from GitHub and applies in Supabase dashboard
- GitHub is the source of truth - not local files, not Supabase directly
- Migration files are numbered: `058_jsonb_parameters.sql`, etc.

---

## Credentials & Secrets

**Where keys live:**

| Secret Type | Location | Notes |
|-------------|----------|-------|
| `SUPABASE_URL` | GitHub Secrets | Bootstrap credential |
| `SUPABASE_SERVICE_KEY` | GitHub Secrets | Service role for admin ops |
| `VAULT_KEY` | GitHub Secrets | Decrypts Supabase vault |
| **All other API keys** | Supabase Vault | Encrypted, retrieved via vault_manager.py |

**Never hardcode secrets.** Use `vault_manager.py` to retrieve at runtime.

---

## Agent Role → Branch Mapping

### System Researcher (Daily Research)
**Branch:** `research-considerations`

**What goes here:**
- Raindrop bookmark research
- GitHub repo analysis
- Technology comparisons
- Update considerations
- Research digests

**Rules:**
- Only documents and findings
- Never touches production code
- Commit message: `Research: YYYY-MM-DD brief description`

**Files:**
- `docs/research/*.md`
- `docs/UPDATE_CONSIDERATIONS.md`
- Analysis scripts

---

### Maintenance Agent (Code Changes)
**Branch:** `main` (for rollbackable changes)

**What goes here:**
- Bug fixes
- Config updates
- New scripts/tools
- Schema changes (with rollback plan)
- Documentation updates

**Rules:**
- Must be rollbackable via git
- No UI/dashboard changes without feature branch
- Commit message: `type: description` (e.g., `fix: rate limit checking`)

---

### Feature Developer (New Features)
**Branch:** `feature/description` (e.g., `feature/agent-dashboard-v2`)

**What goes here:**
- UI changes
- Dashboard updates
- New user-facing features
- Breaking changes

**Rules:**
- Create branch: `git checkout -b feature/description`
- Push to origin regularly
- **NEVER merge to main without human approval**
- Human tests via preview URL
- Human approves → then merge

---

## Start of Session Checklist

**Every agent, every session:**

1. **Read CURRENT_STATE.md** - Know where we are
2. **Check your role** - Researcher? Maintenance? Feature dev?
3. **Checkout correct branch:**
   ```bash
   # System Researcher
   git checkout research-considerations
   git pull origin research-considerations
   
   # Maintenance / Code changes
   git checkout main
   git pull origin main
   
   # Feature development
   git checkout -b feature/your-feature-name
   ```

4. **Verify branch:**
   ```bash
   git branch --show-current
   ```

5. **Check AGENT_CHAT.md (MANDATORY):**
   ```bash
   ./start_session.sh
   # Or manually:
   cat AGENT_CHAT.md
   ./check_chat.sh --once
   ```
   
   **WHY:** Other agents may be waiting for you, have questions, or need coordination. Always check before starting work.

6. **Review AGENT_PROTOCOL.md if unclear:**
   - Communication rules
   - Handoff procedures
   - Response time expectations

---

## Agent-to-Agent Communication Protocol

**Working with other agents (Kimi ↔ GLM-5)? Follow this:**

### Channels (In Priority Order)

| Channel | Use For | Persistence | Check Frequency |
|---------|---------|-------------|-----------------|
| **AGENT_CHAT.md** | Primary communication, planning, council discussions | Git-backed, permanent | **Start of every session** |
| **Supabase** | Urgent/real-time, notifications | 7-day retention, auto-purged | When AGENT_CHAT indicates new messages |
| **Git commits** | Code context, inline comments | Permanent | During code review |

### AGENT_CHAT.md Rules

1. **Keep last 20 messages** (approximate)
2. **Daily backup:** AGENT_CHAT.md → AGENT_CHAT_YYYYMMDD.md at session start
3. **Format:**
   ```markdown
   ### AgentName [YYYY-MM-DD HH:MM] - Subject
   
   Message content...
   
   ---
   ```
4. **Always check at session start:** Other agents may be waiting

### Supabase agent_messages Table

**RLS Policy:** Service role key required (anon key blocked)
**Valid message types:** `chat`, `task`, `alert`
**Usage:**
```python
# Send urgent message
sb.table('agent_messages').insert({
    'from_agent': 'your_name',
    'to_agent': 'target_agent',
    'message_type': 'chat',  # or 'task', 'alert'
    'content': {'text': 'Message...'}
})
```

**Note:** Messages auto-purged after 7 days. AGENT_CHAT.md is the permanent record.

### Before Working on Infrastructure/Code Coordination

1. **Read AGENT_CHAT.md** - Check for messages from other agents
2. **Post your understanding** - "I'm starting work on X, here's my plan..."
3. **Wait for consensus** - Don't proceed if another agent disagrees
4. **Update with progress** - Keep other agents informed

---

## Common Scenarios

### Scenario 1: "I'm researching GitHub repos for VibePilot"
→ **Branch:** `research-considerations`
→ **Action:** Research and commit findings only

### Scenario 2: "I'm fixing a bug in the orchestrator"
→ **Branch:** `main`
→ **Action:** Fix, test, commit, push

### Scenario 3: "I'm updating the documentation"
→ **Branch:** `main`
→ **Action:** Update docs, commit, push

### Scenario 4: "I'm rewriting handler files"
→ **Branch:** `main`
→ **Action:** Rewrite handlers, commit, push
→ **Note:** handlers_plan.go, handlers_task.go, handlers_testing.go, handlers_council.go, handlers_research.go, handlers_maint.go are being rewritten in Sessions 1-4

### Scenario 4: "I'm updating the documentation"
→ **Branch:** `main`
→ **Action:** Update docs, commit, push

---

## Emergency Procedures

### "I committed to wrong branch"

**If not pushed yet:**
```bash
git reset HEAD~1  # Undo last commit
git stash          # Save changes
git checkout correct-branch
git stash pop      # Apply changes
git add .
git commit -m "..."
```

**If already pushed:**
```bash
# Tell human immediately
# Do NOT try to fix with force push
# Human will handle it
```

### "Branch has conflicts"
```bash
# Pull latest first
git pull origin branch-name

# If conflicts, tell human
# Do NOT resolve complex conflicts without human guidance
```

---

## Branch Protection Rules

**main:**
- Direct pushes allowed for rollbackable code
- UI changes → feature branch required

**research-considerations:**
- Any agent can push research docs
- No code changes (enforced by convention)

**feature/*:**
- Create freely
- Push freely
- Merge ONLY with human approval

---

## Mental Model for VibePilot Improvements

**How to think when proposing changes:**

### Research First
- Understand what exists before building new
- Learn from best solutions (ZeroClaw, NanoClaw, IronClaw, etc.)
- Copy patterns, customize to our context

### Think Backwards from Future Problems
- Don't just fix today's bug
- Ask: "Will this cause tomorrow's problem?"
- Design to prevent, not just react

### Principles Over Preferences
- Swappable, modular, lean, no lock-in
- 1 file = 1 concern (changes touch one file max)
- Config-driven, not code-driven
- Track every token, every model, every task

### Clean As You Go
- Simplify, don't accumulate
- 4k lines is achievable (NanoClaw proved it)
- Code should fit in LLM context for easy modification
- Remove dead code, don't just add new code

### The Claw Framework Lessons

| From | Pattern | For VibePilot |
|------|---------|---------------|
| **ZeroClaw** | Provider traits | Config-driven LLM swapping |
| **ZeroClaw** | 5MB footprint | Free tier compatible |
| **ZeroClaw** | SQLite only | Remove abstraction layers |
| **NanoClaw** | ~4k lines | Fits in LLM context |
| **NanoClaw** | 1 file per concern | Easy to modify |
| **NanoClaw** | Skills over plugins | Transform codebase, don't configure |
| **IronClaw** | Leak detection | Scan tool outputs for secrets |
| **IronClaw** | Credential injection | Secrets at boundary, never in context |

### AI Coding Time Reality
- 5 days = ~1 day actual coding
- Subscription already paid = marginal cost is $0
- "Weeks of work" = hours with AI
- The only constraints: human decisions, getting it right

### Before Proposing Solutions
1. **Research** - What exists? What patterns are proven?
2. **Audit** - What do we actually have vs dead weight?
3. **Apply patterns** - Customize proven solutions
4. **Verify lean** - Does it fit free tier?
5. **Think future** - Will this scale? Swap? Evolve?

---

## Questions?

**Not sure which branch?** → Ask the human
**Unsure if rollbackable?** → Use feature branch
**Dashboard/UI involved?** → Feature branch
**Just docs/research?** → research-considerations or main

---

## Commit Message Format

```
Type: Brief description

Longer explanation if needed.

Closes #123 (if applicable)
```

**Types:**
- `Research:` - Research findings
- `Fix:` - Bug fixes
- `Feat:` - New features
- `Docs:` - Documentation
- `Refactor:` - Code restructuring
- `Config:` - Configuration changes

---

## Remember:
- **When in doubt, ask the human**
- **UI changes → feature branch, always**
- **Research → research-considerations**
- **Code fixes → main (if rollbackable)**

---

## After Every Session: Update Documentation

**MANDATORY updates before ending a session:**

| Document | When to Update | What to Update |
|----------|----------------|----------------|
| `ARCHITECTURE.md` | Architecture changed | Architecture, flow, components, config |
| `CURRENT_STATE.md` | Always | What was done, what's next, files changed |
| `CHANGELOG.md` | Always | Full audit trail (date, what, files, commits) |
| `SESSION_HANDOFF.md` | Major work | Critical context for next session |

**Commit documentation changes separately:**
```bash
git add ARCHITECTURE.md CURRENT_STATE.md CHANGELOG.md
git commit -m "docs: update architecture, current state, changelog for session XX"
git push origin main
```

---

## Quick Links

**Required Reading (Every Session):**
| Document | Purpose |
|----------|---------|
| [VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md](VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md) | Everything you need |
| [CURRENT_STATE.md](CURRENT_STATE.md) | What's done, what's next |

**Reference When Needed:**
| Document | Purpose | Read When |
|----------|---------|-----------|
| [CHANGELOG.md](CHANGELOG.md) | Full history | Need context on changes |
| [docs/GO_REwrite_SPEC.md](docs/GO_REWRITE_SPEC.md) | Rewrite specification | During rewrite sessions |
| [docs/HOW_DASHBOARD_WORKS.md](docs/HOW_DASHBOARD_WORKS.md) | Dashboard data flow | Fixing dashboard issues |
| [docs/DATA_FLOW_MAPPING.md](docs/DATA_FLOW_MAPPING.md) | Dashboard → Supabase → Go mapping | Understanding data flow |
| [docs/core_philosophy.md](docs/core_philosophy.md) | Strategic mindset | Need to understand "why" |
| [docs/supabase-schema/](docs/supabase-schema/) | Database schema | Making schema changes |
