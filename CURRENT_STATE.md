# VibePilot Current State

**Required reading: SEVEN files**
1. **THIS FILE** (`CURRENT_STATE.md`) - What, where, how, current state
2. **`AUDIT_REPORT.md`** - FULL CODE AUDIT - What works, what doesn't, what's missing
3. **`docs/vibepilot_process.md`** - COMPLETE process flow, all roles, failure handling, learning
4. **`docs/learning_system.md`** - Learning system design, review flow, thresholds
5. **`docs/SYSTEM_REFERENCE.md`** - What we have and how it works
6. **`docs/SESSION_35_HANDOFF.md`** - Dynamic routing implementation details
7. **`docs/core_philosophy.md`** - Strategic mindset and principles

**Read all six → Know everything → Do anything**

---

**Last Updated:** 2026-02-28
**Updated By:** GLM-5 - Session 37
**Branch:** `main`
**Status:** ACTIVE - Decision parsing implemented, PRD detection wired

---

# CURRENT ARCHITECTURE

## What's Running

```
vibepilot-governor.service (Go binary)
├── Polls Supabase every 1s
├── Max 8 concurrent per module, 160 total
├── Dynamic routing via config (NO hardcoded destinations)
├── Branch creation when Orchestrator assigns task
├── Reads secrets from vault at runtime
├── Startup recovery: finds and recovers orphaned sessions
├── Usage tracking: multi-window rate limit enforcement
└── Learning: model scoring RPC (ready to deploy)
```

**Status:** `systemctl status vibepilot-governor`

## Architecture Principle

```
Model = Intelligence (thinks, outputs)
Transport/CLI = Provides tools natively (read/write/bash)
Destination = Where/how access happens (has capabilities)
Agent = Role with capabilities needed (for routing)
Prompt packet = Task + expected output format
Hat = Prompt/role a model wears for a specific task

Routing = config/routing.json (strategies, restrictions, categories)
Destinations = config/destinations.json (status, type, provides_tools)
Models = config/models.json (availability, access_via)

NO HARDCODED DESTINATIONS. Everything configurable.
ALL CHANGES GO THROUGH TASK SYSTEM. Nothing implemented directly.
```

## Dynamic Routing

```
Event fires
    ↓
selectDestination(agentID, taskID, taskType)
    ↓
Get strategy for agent (internal_only for governance agents)
    ↓
Get priority order from routing.json
    ↓
For each category: find active destination
    ↓
Get model score from RPC (success_rate from task_runs)
    ↓
Return destination ID or "" if none available
```

**Internal agents (planner, supervisor, council, etc.)** → internal_only strategy → never external
**Task execution** → default strategy → external first, then internal

## "Hats" Concept

Models don't have fixed roles. Orchestrator assigns any available model to wear the appropriate "hat" (use the right prompt) for each task.

Example:
- Task needs maintenance work → Orchestrator picks available model → Model wears "maintenance hat"
- Task needs planning → Orchestrator picks available model → Model wears "planner hat"

## Codebase Structure (Clean)

```
vibepilot/
├── governor/              # ACTIVE - Go binary (everything)
│   ├── cmd/governor/      # Main entry point + event handlers + routing
│   ├── internal/
│   │   ├── db/            # Supabase client + RPC allowlist
│   │   ├── vault/         # Secret decryption
│   │   ├── runtime/       # Events, sessions, router, usage_tracker
│   │   ├── gitree/        # Git operations (branch, commit, merge, delete)
│   │   ├── destinations/  # CLI/API runners
│   │   └── tools/         # Tool registry
│   └── config/            # JSON configs (routing.json, destinations.json, etc.)
├── prompts/               # Agent behavior definitions (.md)
├── docs/                  # Documentation
├── research/              # Review docs for human (on research branch)
├── scripts/               # Deploy scripts
└── legacy/                # DEAD CODE - kept for reference
```

---

## Bootstrap Keys (Secure)

| Key | Where It Lives | Who Can Read |
|-----|----------------|--------------|
| `SUPABASE_URL` | `/etc/systemd/.../override.conf` | root only |
| `SUPABASE_SERVICE_KEY` | `/etc/systemd/.../override.conf` | root only |
| `VAULT_KEY` | `/etc/systemd/.../override.conf` | root only |

**All other secrets** → Encrypted in Supabase `secrets_vault` table

**Read `docs/SECURITY_BOOTSTRAP.md` before touching credentials.**

---

## Quick Commands

| Command | Action |
|---------|--------|
| `systemctl status vibepilot-governor` | Check if running |
| `journalctl -u vibepilot-governor -f` | Live logs |
| `sudo systemctl restart vibepilot-governor` | Restart |
| `cd ~/vibepilot/governor && go build -o governor ./cmd/governor` | Build |
| `sudo scripts/deploy-governor.sh` | Full deploy |

---

## Session Progress

### DONE - Session 35
- ✅ Dynamic routing (router.go, routing.json)
- ✅ Python moved to legacy/
- ✅ TOOL: format removed

### DONE - Session 36
- ✅ Full documentation update (vibepilot_process.md)
- ✅ Failure handling flow documented
- ✅ Learning system documented (docs/learning_system.md)
- ✅ Branch creation on orchestrator assignment (main.go)
- ✅ Research branch created for review docs
- ✅ "What I've Learned" sections added to agent prompts
- ✅ Supervisor decision matrix (simple/complex/human)
- ✅ System Researcher flow documented
- ✅ Branch lifecycle documented
- ✅ Courier vs Internal clarified
- ✅ "Hats" concept documented
- ✅ All changes go through task system documented
- ✅ Model scoring RPC created (033_model_scoring_rpc.sql)
- ✅ Model scoring added to RPC allowlist
- ✅ FULL CODE AUDIT COMPLETE

### AUDIT FINDINGS (Session 36)

**What works:**
- Config loading, dynamic routing, event polling, vault, gitree library

**What's missing (CRITICAL):**
- Decision parsing - Governor doesn't parse agent JSON outputs
- State transitions - No status updates based on decisions
- Rejection handling - No wipe/reassign/escalate logic
- GitHub commits - gitree.CommitOutput exists but never called
- PRD detection - github client exists but not wired

**Verdict:** Infrastructure works. Brain is missing.

### READY TO DEPLOY

| Step | Action | File/Command |
|------|--------|--------------|
| 1 | Run migration in Supabase | `docs/supabase-schema/033_model_scoring_rpc.sql` |
| 2 | Deploy governor | `sudo scripts/deploy-governor.sh` |

### DONE - Session 37
- ✅ decision.go - Parse agent outputs
- ✅ context_builder.go - Build context from existing RPCs
- ✅ prd_watcher.go - Detect new PRDs
- ✅ EventTaskReview - Parse decision, call record_failure, update status
- ✅ EventPRDReady - Parse planner output, commit to GitHub
- ✅ EventPlanReview - Parse initial review, set status
- ✅ EventCouncilDone - Parse votes, set consensus, create planner rules
- ✅ EventTestResults - Parse outcome, merge/reset/await based on result
- ✅ Context builder wired to planner and supervisor sessions
- ✅ PRD watcher wired to main.go
- ✅ All changes use existing RPCs, no new tables

### CRITICAL ITEMS - ALL DONE
- ✅ Wire Council output → set_council_consensus
- ✅ Wire test results → update status + merge + unlock
- ✅ Context builder to session.go

### NEXT - Session 38+

| Priority | Task | Notes |
|----------|------|-------|
| **CRITICAL** | Decision parser | Parse agent JSON outputs in main.go |
| **CRITICAL** | State transitions | Update Supabase status based on decisions |
| **CRITICAL** | Rejection handler | Wipe branch, count failures, reassign/escalate |
| **CRITICAL** | Wire gitree.CommitOutput | Actually commit runner output to GitHub |
| **CRITICAL** | Wire PRD detection | Poll GitHub, create plan records |
| HIGH | Rate limit checking | Router checks destination limits |
| MEDIUM | API output execution | Governor parses and executes for API runners |
| LOW | Courier runner | Web platform execution implementation |

---

## For Next Session

**READ AUDIT_REPORT.md FIRST.**

**Critical Priority: Build the brain**

The Governor needs to:
1. **Parse agent outputs** - Extract decision JSON from agent response
2. **Take action** - Update status, wipe branches, reassign, escalate
3. **Commit to GitHub** - Call gitree.CommitOutput with runner output
4. **Detect PRDs** - Poll GitHub for new PRDs, create plan records

**Do NOT add more infrastructure.** Build the decision loop.

---

## Key Principles

- **All changes go through task system** - Nothing implemented directly
- **Hats, not fixed roles** - Any model can wear any hat
- **Everything configurable** - Nothing hardcoded
- **Everything learns** - All agents improve over time
- **Human reviews only** complex suggestions, API credit, UI/UX

---

## What NOT to Do

- Don't look for keys in `.env` (it's empty)
- Don't use Python code (it's in legacy/)
- Don't hardcode keys anywhere
- Don't hardcode destination IDs in code
- Don't hardcode routing logic in code
- Don't add TOOL: format back - it's gone for good
- Don't modify cleanup script without understanding cgroup logic
- Don't hardcode any defaults - everything in config files
- Don't implement anything directly - all changes through task system
