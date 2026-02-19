# VibePilot Agent Workflow Guide

**For ALL agents/LLMs working on VibePilot**

---

## Quick Branch Reference

| Branch | Use For | Who Can Push | Approval Needed |
|--------|---------|--------------|-----------------|
| `main` | Backend code, scripts, configs, docs (rollbackable) | Any agent | No (if rollbackable) |
| `research-considerations` | Research findings, analysis, considerations | System Researcher | No (docs only) |
| `feature/*` | UI/dashboard changes, major features | Any agent | **YES - Human must approve** |
| `hotfix/*` | Emergency fixes | Any agent | After fix (retroactive) |

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

## Common Scenarios

### Scenario 1: "I'm researching GitHub repos for VibePilot"
→ **Branch:** `research-considerations`
→ **Action:** Research and commit findings only

### Scenario 2: "I'm fixing a bug in the orchestrator"
→ **Branch:** `main`
→ **Action:** Fix, test, commit, push

### Scenario 3: "I'm improving the dashboard CSS"
→ **Branch:** `feature/dashboard-css-fix`
→ **Action:** Create branch, make changes, push, **wait for human approval**

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

**Remember:**
- **When in doubt, ask the human**
- **UI changes → feature branch, always**
- **Research → research-considerations**
- **Code fixes → main (if rollbackable)**
